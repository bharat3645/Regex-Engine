// File: core/scan_manager.go
package core

import (
	"Regex/checksum"
	"Regex/config"
	"Regex/connectors"
	"Regex/database"
	"Regex/engine"
	"Regex/events"
	"Regex/lineage"
	"Regex/logger"
	"Regex/ocr"
	"Regex/pipeline"
	"Regex/rules"
	"Regex/types"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	godebug "runtime/debug"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type ScanManager struct {
	logger       *logger.AppLogger
	db           *database.Manager
	emitter      events.EventEmitter
	cfg          *config.Config
	isScanActive atomic.Bool
	isPaused     atomic.Bool
	isOCREnabled atomic.Bool // NEW: To toggle OCR scanning
	cancelScan   context.CancelFunc
	orchestrator *pipeline.Orchestrator
	ocrManager   *ocr.OCRManager // NEW: The manager for the Python process
}

func NewScanManager(appLogger *logger.AppLogger, db *database.Manager, emitter events.EventEmitter) *ScanManager {
	return &ScanManager{
		logger:  appLogger,
		db:      db,
		emitter: emitter,
	}
}

// SetOCREnabled allows the main app to toggle the OCR feature.
func (sm *ScanManager) SetOCREnabled(enabled bool) {
	sm.isOCREnabled.Store(enabled)
}

func (sm *ScanManager) Start() {
	if !sm.isScanActive.CompareAndSwap(false, true) {
		sm.logger.Warn("Scan is already in progress.")
		return
	}
	sm.isPaused.Store(false)

	go func() {
		defer sm.isScanActive.Store(false)

		cfg, err := config.Load()
		if err != nil {
			sm.HandleError(fmt.Sprintf("CRITICAL ERROR: Failed to load config.json: %v", err))
			return
		}
		sm.cfg = cfg

		start := time.Now()
		scanCtx, cancel := context.WithCancel(context.Background())
		sm.cancelScan = cancel
		defer cancel()

		sm.emitter.Emit("scan:starting")

		rulesChanged, err := sm.checkRulesIntegrity(scanCtx)
		if err != nil {
			sm.HandleError(fmt.Sprintf("Failed to check rules integrity: %v", err))
			return
		}
		if rulesChanged {
			sm.emitter.Emit("log:message", &types.LogEntry{Level: "INFO", Msg: "Rules updated, starting full scan."})
		} else {
			sm.emitter.Emit("log:message", &types.LogEntry{Level: "INFO", Msg: "Rules unchanged, starting incremental scan."})
		}

		if err := sm.db.ClearJobs(); err != nil {
			sm.HandleError(fmt.Sprintf("Failed to clear database: %v", err))
			return
		}

		totalFiles, err := sm.runDiscoveryStage(scanCtx)
		if err != nil {
			if scanCtx.Err() != context.Canceled {
				sm.HandleError(fmt.Sprintf("Discovery failed: %v", err))
			} else {
				sm.HandleCancellation()
			}
			return
		}

		err = sm.runProcessingStage(scanCtx, totalFiles)
		if err != nil {
			if scanCtx.Err() != context.Canceled {
				sm.HandleError(fmt.Sprintf("Processing failed: %v", err))
			} else {
				sm.HandleCancellation()
			}
			return
		}

		if scanCtx.Err() == nil {
			sm.logger.Info("Scan completed successfully", "duration", time.Since(start), "files_scanned", totalFiles)
			sm.emitter.Emit("scan:complete", types.ScanCompletePayload{FilesScanned: totalFiles})
		} else {
			sm.HandleCancellation()
		}
	}()
}

func (sm *ScanManager) PauseScan() {
	if sm.isScanActive.Load() && !sm.isPaused.Load() {
		sm.isPaused.Store(true)
		sm.logger.Info("Scan pause signal received. Pipeline will idle after processing queued files.")
		sm.emitter.Emit("scan:paused")
	}
}

func (sm *ScanManager) ResumeScan() {
	if sm.isScanActive.Load() && sm.isPaused.Load() {
		sm.isPaused.Store(false)
		sm.logger.Info("Scan resume signal received.")
		sm.emitter.Emit("scan:resumed")
	}
}

func (sm *ScanManager) Stop() {
	if sm.isScanActive.Load() && sm.cancelScan != nil {
		sm.logger.Info("Stop scan signal received.")
		sm.cancelScan()
	}
}

func (sm *ScanManager) GetQueueDepth() int {
	if sm.orchestrator != nil {
		return sm.orchestrator.GetTotalQueueSize()
	}
	return 0
}

func (sm *ScanManager) checkRulesIntegrity(ctx context.Context) (bool, error) {
	rulesPath, err := config.GetPath(sm.cfg.RulesDir)
	if err != nil {
		return false, err
	}

	files, err := os.ReadDir(rulesPath)
	if err != nil {
		return false, err
	}

	hasher := sha256.New()
	var filePaths []string
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			filePaths = append(filePaths, filepath.Join(rulesPath, file.Name()))
		}
	}
	sort.Strings(filePaths) // Ensure consistent order

	for _, path := range filePaths {
		err := func() error {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(hasher, f); err != nil {
				return err
			}
			return nil
		}()
		if err != nil {
			return false, err
		}
	}

	currentRulesSum := hex.EncodeToString(hasher.Sum(nil))
	storedRulesSum, err := sm.db.GetMetadata(ctx, "rules_checksum")
	if err != nil {
		return false, err
	}

	if currentRulesSum != storedRulesSum {
		if err := sm.db.ClearFileCache(ctx); err != nil {
			return true, err
		}
		return true, sm.db.SetMetadata(ctx, "rules_checksum", currentRulesSum)
	}

	return false, nil
}

func (sm *ScanManager) runDiscoveryStage(ctx context.Context) (int64, error) {
	sm.logger.Info("Starting Stage 1: File Discovery")
	sm.emitter.Emit("scan:progress", types.ProgressUpdate{Message: "Stage 1: Discovering files to scan..."})

	discoveryChan := make(chan types.FileJob, sm.cfg.PipelineBufferSize)

	// NEW: Use Connectors Framework via Factory
	// This makes the agent truly universal (Local vs S3 vs DB)
	factory := connectors.NewFactory(sm.db, sm.logger)
	connector, err := factory.GetConnector(sm.cfg.DataSourceType)
	if err != nil {
		sm.logger.Error("Failed to initialize connector", "type", sm.cfg.DataSourceType, "error", err)
		return 0, err
	}

	sm.logger.Info("Using Data Connector", "name", connector.Name())

	go func() {
		// Initialize connection (noop for local_fs, but critical for S3/DB)
		if err := connector.Connect(ctx, sm.cfg.DataSourceConfig); err != nil {
			sm.logger.Error("Connector connection failed", "error", err)
			close(discoveryChan) // Close explicitly to avoid deadlock if walking never starts
			return
		}

		if err := connector.Walk(ctx, sm.cfg.RootDir, discoveryChan); err != nil {
			sm.logger.Error("Discovery connector failed", "error", err)
		}
	}()

	var fileCount int64
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	batch := make([]types.FileJob, 0, sm.cfg.DiscoveryBatchSize)

	for {
		select {
		case <-ctx.Done():
			for range discoveryChan {
			}
			return fileCount, ctx.Err()
		case <-ticker.C:
			sm.emitter.Emit("scan:progress", types.ProgressUpdate{Message: fmt.Sprintf("Discovered %d files...", fileCount)})
		case job, ok := <-discoveryChan:
			if !ok {
				if err := sm.db.AddJobsBatch(ctx, batch); err != nil {
					sm.logger.Error("Failed to save final discovery batch to DB", "error", err)
				}
				sm.logger.Info("Stage 1: Discovery complete", "total_files", fileCount)
				return fileCount, nil
			}
			batch = append(batch, job)
			fileCount++
			if len(batch) >= sm.cfg.DiscoveryBatchSize {
				if err := sm.db.AddJobsBatch(ctx, batch); err != nil {
					sm.logger.Error("Failed to save discovery batch to DB", "error", err)
				}
				batch = batch[:0]
			}
		}
	}
}

func (sm *ScanManager) runProcessingStage(ctx context.Context, totalFiles int64) error {
	if totalFiles == 0 {
		sm.logger.Info("No new or modified files found to process. Skipping Stage 2.")
		return nil
	}

	sm.logger.Info("Starting Stage 2: Extraction and Scanning")
	sm.emitter.Emit("scan:progress", types.ProgressUpdate{Message: "Stage 2: Preparing to scan files..."})

	rulesPath, err := config.GetPath(sm.cfg.RulesDir)
	if err != nil {
		return fmt.Errorf("could not determine rules path: %w", err)
	}
	loadedRules, err := rules.LoadFromFolder(rulesPath)
	if err != nil {
		return fmt.Errorf("failed to load compliance rules: %w", err)
	}
	scanEngine, err := engine.NewEngine(loadedRules, sm.logger)
	if err != nil {
		return fmt.Errorf("failed to create regex engine: %w", err)
	}

	// The orchestrator now gets the OCR status flag.
	sm.orchestrator = pipeline.NewOrchestrator(sm.cfg, sm.logger, sm.db, scanEngine)
	var filesScanned atomic.Int64
	resultsChan, processedJobsChan, processingWg, ocrChan := sm.orchestrator.StartProcessing(ctx, totalFiles, &sm.isPaused, &sm.isOCREnabled, &filesScanned)

	// Start the OCR Manager only if the feature is enabled.
	if sm.isOCREnabled.Load() {
		sm.ocrManager = ocr.NewOCRManager(sm.logger, sm.orchestrator.GetScannerChan(), sm.cfg.OCRPort)
		// Connect the orchestrator's OCR output to the OCR manager's input.
		go func() {
			for job := range ocrChan {
				sm.ocrManager.OcrJobsChan <- job
			}
			close(sm.ocrManager.OcrJobsChan)
		}()
		if err := sm.ocrManager.Start(ctx); err != nil {
			return fmt.Errorf("failed to start OCR Manager: %w", err)
		}
		defer sm.ocrManager.Stop()
	}

	var aggregatorWg, cacheWg sync.WaitGroup
	aggregatorWg.Add(1)
	go func() {
		defer aggregatorWg.Done()
		outputPath, _ := config.GetPath(sm.cfg.OutputFile)
		sm.saveResults(ctx, resultsChan, outputPath)
	}()

	cacheWg.Add(1)
	go func() {
		defer cacheWg.Done()
		for job := range processedJobsChan {
			info, err := os.Stat(job.Path)
			if err != nil {
				continue
			}
			chk, err := checksum.CalculateFileChecksum(job.Path)
			if err != nil {
				continue
			}
			entry := database.FileCacheEntry{
				Path:         job.Path,
				Checksum:     chk,
				ModifiedTime: info.ModTime().Unix(),
			}
			sm.db.UpdateFileCacheEntry(ctx, entry)
		}
	}()

	progressDone := make(chan struct{})
	go sm.monitorProgress(ctx, &filesScanned, totalFiles, progressDone)

	processingWg.Wait()

	if ctx.Err() == nil {
		sm.logger.Info("All files processed. Finalizing results and updating cache...")
		sm.emitter.Emit("scan:progress", types.ProgressUpdate{
			FilesScanned: totalFiles,
			TotalFiles:   totalFiles,
			Message:      "Finalizing results & Analyzing Lineage...",
		})

		// NEW: Run Lineage Linker (Post-Processing)
		linker := lineage.NewLinker(sm.db, sm.logger)
		if err := linker.FindExactDuplicates(ctx); err != nil {
			sm.logger.Error("Lineage analysis failed", "error", err)
		}
	}

	aggregatorWg.Wait()
	cacheWg.Wait()
	<-progressDone
	return nil
}

func (sm *ScanManager) monitorProgress(ctx context.Context, filesScanned *atomic.Int64, totalFiles int64, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	var lastGCRun int64

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			scanned := filesScanned.Load()
			if totalFiles > 0 && scanned >= totalFiles {
				sm.emitter.Emit("scan:progress", types.ProgressUpdate{
					FilesScanned: totalFiles,
					TotalFiles:   totalFiles,
					Message:      fmt.Sprintf("Stage 2: Scanned %d / %d files", totalFiles, totalFiles),
				})
				return
			}

			sm.emitter.Emit("scan:progress", types.ProgressUpdate{
				FilesScanned: scanned,
				TotalFiles:   totalFiles,
				Message:      fmt.Sprintf("Stage 2: Scanned %d / %d files", scanned, totalFiles),
			})

			if sm.cfg.GCTriggerMB > 0 {
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				memAllocMB := m.Alloc / (1024 * 1024)
				if memAllocMB > uint64(sm.cfg.GCTriggerMB) && (scanned-lastGCRun) > 5000 {
					sm.logger.Info("Governor: Memory threshold reached, triggering GC.", "mem_alloc_mb", memAllocMB)
					godebug.FreeOSMemory()
					lastGCRun = scanned
				}
			}
		}
	}
}
func (sm *ScanManager) saveResults(ctx context.Context, results <-chan types.RiskTag, outputPath string) {
	// The "Spoke" Agent logic:
	// 1. Receive Tags from Scanner
	// 2. Persist to Local DB (pii_tags table)
	// 3. (Optional) Append to a JSON report for local debugging, but primary store is DB.

	// For this evolution step, we will write to DB.
	sm.logger.Info("Result Aggregator started. Saving tags to local database.")

	for tag := range results {
		if err := sm.db.AddRiskTag(ctx, tag); err != nil {
			sm.logger.Error("Failed to save risk tag to DB", "node_id", tag.NodeID, "rule", tag.RuleName, "error", err)
		}
	}
	sm.logger.Info("Result Aggregator finished.")
}

func (sm *ScanManager) HandleError(message string) {
	sm.logger.Error(message)
	sm.emitter.Emit("scan:error", message)
}

func (sm *ScanManager) HandleCancellation() {
	sm.logger.Info("Scan was cancelled by user.")
	sm.emitter.Emit("scan:stopped")
}
