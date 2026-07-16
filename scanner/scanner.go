// File: scanner/scanner.go
package scanner

import (
	"context"
	"io"
	"sync"
	"sync/atomic"

	"Regex/client" // For MLClient (Moved from core)
	"Regex/config"
	"Regex/engine"
	"Regex/types"
)

const (
	overlapSize = 1024
)

// Start runs a pool of scanner workers.
// Start runs a pool of scanner workers with ML Client
func Start(ctx context.Context, cfg *config.Config, scanEngine *engine.Engine, jobs <-chan types.ExtractedContent, results chan<- types.RiskTag, progress *atomic.Int64, wg *sync.WaitGroup) {
	// Initialize ML Client (Server-Side Logic)
	mlClient := client.NewMLClient(cfg.MLServerURL, nil) // Logger nil for now or pass in

	// Pre-flight Health Check
	if serverUp, err := mlClient.HealthCheck(ctx); !serverUp || err != nil {
		// Log warning (assuming we don't have logger passed directly, we might print or ignore)
		// In a real app, we should probably output this to the Application Logger.
		// For now, we proceed but expect errors if the server is truly down.
		// fmt.Printf("WARN: ML Server not reachable: %v\n", err)
	}

	for i := 1; i <= cfg.ScannerWorkers; i++ {
		go worker(ctx, cfg, scanEngine, mlClient, jobs, results, progress, wg)
	}
}

func worker(ctx context.Context, cfg *config.Config, scanEngine *engine.Engine, mlClient *client.MLClient, jobs <-chan types.ExtractedContent, results chan<- types.RiskTag, progress *atomic.Int64, wg *sync.WaitGroup) {
	defer wg.Done()

	chunkSize := cfg.ScannerBufferSizeMB * 1024 * 1024
	buffer := make([]byte, chunkSize+overlapSize)

	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-jobs:
			if !ok {
				return
			}

			func() {
				if job.Closer != nil {
					defer job.Closer.Close()
				}
				defer progress.Add(1)

				reader := job.ContentStream
				overlap := make([]byte, 0, overlapSize)

				for {
					select {
					case <-ctx.Done():
						return
					default:
					}

					copy(buffer, overlap)
					bytesRead, err := io.ReadFull(reader, buffer[len(overlap):chunkSize])
					totalBytes := len(overlap) + bytesRead

					if totalBytes == 0 {
						break
					}

					scanText := string(buffer[:totalBytes])

					matches := scanEngine.Scan(scanText)

					// Prepare batch for ML Server
					candidates := make([]client.PIIContext, 0, len(matches))

					for _, match := range matches {
						// Extract Context Window (e.g., 50 chars before and after)
						const contextRadius = 50
						windowStart := match.StartIndex - contextRadius
						if windowStart < 0 {
							windowStart = 0
						}
						windowEnd := match.EndIndex + contextRadius
						if windowEnd > len(scanText) {
							windowEnd = len(scanText)
						}

						contextStr := scanText[windowStart:windowEnd]

						// Calculate indexes relative to the window
						relStart := match.StartIndex - windowStart
						relEnd := match.EndIndex - windowStart

						candidates = append(candidates, client.PIIContext{
							TextSegment: contextStr,
							StartIndex:  relStart,
							EndIndex:    relEnd,
							FilePath:    job.Path,
							Metadata: map[string]interface{}{
								"rule_id":  match.RuleID,
								"pii_type": match.RuleName,
							},
						})
					}

					if len(candidates) > 0 {
						// Call ML Server
						validated, err := mlClient.ScanCandidates(ctx, candidates)
						if err == nil {
							for _, v := range validated {
								if v.IsValid {
									results <- types.RiskTag{
										NodeID:           job.NodeID,
										RuleID:           "ML_VALIDATED",
										RuleName:         v.PIIType,
										RiskScore:        v.ConfidenceScore, // Use confidence as risk for now
										Count:            1,
										ConfidenceScore:  v.ConfidenceScore,
										VectorEmbedding:  v.VectorEmbedding,
										ValidationStatus: "validated",
										PIITypeRefined:   v.PIIType,
									}
								}
							}
						}
					}

					if err == io.ErrUnexpectedEOF || err == io.EOF {
						break
					}
					if err != nil {
						scanEngine.Logger.Warn("Error reading file chunk", "path", job.Path, "error", err)
						break
					}

					if totalBytes > overlapSize {
						overlap = append(overlap[:0], buffer[totalBytes-overlapSize:totalBytes]...)
					} else {
						overlap = append(overlap[:0], buffer[:totalBytes]...)
					}
				}
			}()
		}
	}
}
