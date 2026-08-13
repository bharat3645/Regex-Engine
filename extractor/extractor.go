// File: extractor/extractor.go
package extractor

import (
	"github.com/bharat3645/compliance-manager/config"
	"github.com/bharat3645/compliance-manager/logger"
	"github.com/bharat3645/compliance-manager/types"
	"context"
	"io"
	"os"
	"sync"
	"sync/atomic"
)

// StartWorkers now accepts the ocrJobsChan and isOcrEnabled flag to route PDFs and images.
func StartWorkers(ctx context.Context, cfg *config.Config, jobs <-chan types.FileJob, results chan<- types.ExtractedContent, ocrJobsChan chan<- types.FileJob, processedJobs chan<- types.FileJob, isOcrEnabled *atomic.Bool, wg *sync.WaitGroup, appLogger *logger.AppLogger) {
	defer wg.Done()

	numWorkers := cfg.ExtractorWorkers
	var workerWg sync.WaitGroup
	appLogger.Info("Unified Go extractor pool started", "workers", numWorkers)

	for i := 0; i < numWorkers; i++ {
		workerWg.Add(1)
		go func() {
			defer workerWg.Done()
			for job := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				func(currentJob types.FileJob) {
					// File size check
					info, err := os.Stat(currentJob.Path)
					if err != nil {
						appLogger.Warn("Extractor failed: could not stat file", "path", currentJob.Path, "error", err)
						processedJobs <- currentJob
						return
					}
					if info.Size() > int64(cfg.MaxFileSizeMB)*1024*1024 {
						appLogger.Info("Skipping large file", "path", currentJob.Path, "size_mb", info.Size()/(1024*1024))
						processedJobs <- currentJob
						return
					}

					var stream io.ReadCloser
					// --- REVISED OCR ROUTING LOGIC ---
					if isOcrEnabled.Load() {
						switch currentJob.Extension {
						case ".pdf":
							isImage, err := isImageBasedPDF(currentJob.Path)
							if err != nil {
								appLogger.Warn("PDF analysis failed, treating as text-based", "path", currentJob.Path, "error", err)
							}
							if isImage {
								appLogger.Info("Image-based PDF detected, routing to OCR", "path", currentJob.Path)
								ocrJobsChan <- currentJob
								processedJobs <- currentJob
								return
							}
						case ".png", ".jpg", ".jpeg", ".tiff", ".bmp", ".gif":
							appLogger.Info("Image file detected, routing to OCR", "path", currentJob.Path)
							ocrJobsChan <- currentJob
							processedJobs <- currentJob
							return
						}
					}
					// --- END OCR ROUTING LOGIC ---

					switch currentJob.Extension {
					case ".docx":
						stream, err = extractTextFromDocx(ctx, currentJob.Path)
					case ".xlsx":
						stream, err = extractTextFromXlsx(ctx, currentJob.Path)
					case ".pdf":
						stream, err = extractTextFromPdf(ctx, currentJob.Path)
					default:
						// Images will be skipped here if OCR is disabled, which is correct.
						stream, err = extractPlainText(ctx, currentJob.Path)
					}

					if err != nil {
						appLogger.Warn("Extractor failed, skipping file", "path", currentJob.Path, "error", err)
						processedJobs <- currentJob
						return
					}

					select {
					case results <- types.ExtractedContent{FileJob: currentJob, ContentStream: stream, Closer: stream}:
						processedJobs <- currentJob
					case <-ctx.Done():
						stream.Close()
						return
					}
				}(job)
			}
		}()
	}

	workerWg.Wait()
	close(processedJobs)
}
