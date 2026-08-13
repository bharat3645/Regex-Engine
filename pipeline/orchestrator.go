// File: pipeline/orchestrator.go
package pipeline

import (
	"github.com/bharat3645/compliance-manager/config"
	"github.com/bharat3645/compliance-manager/database"
	"github.com/bharat3645/compliance-manager/dispatcher"
	"github.com/bharat3645/compliance-manager/engine"
	"github.com/bharat3645/compliance-manager/extractor"
	"github.com/bharat3645/compliance-manager/governor"
	"github.com/bharat3645/compliance-manager/logger"
	"github.com/bharat3645/compliance-manager/scanner"
	"github.com/bharat3645/compliance-manager/types"
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Orchestrator manages the entire file processing pipeline.
type Orchestrator struct {
	cfg        *config.Config
	logger     *logger.AppLogger
	db         *database.Manager
	engine     *engine.Engine
	dispatcher *dispatcher.JobDispatcher
	channels   *Channels // NEW: Hold a reference to the channels struct
}

// NewOrchestrator creates a new pipeline orchestrator.
func NewOrchestrator(cfg *config.Config, logger *logger.AppLogger, db *database.Manager, engine *engine.Engine) *Orchestrator {
	return &Orchestrator{
		cfg:    cfg,
		logger: logger,
		db:     db,
		engine: engine,
	}
}

// GetScannerChan provides access to the scanner channel for the OCR manager.
func (o *Orchestrator) GetScannerChan() chan<- types.ExtractedContent {
	if o.channels != nil {
		return o.channels.Scanner
	}
	return nil
}

// StartProcessing now accepts the OCR status flag and returns the new OCR channel.
func (o *Orchestrator) StartProcessing(ctx context.Context, totalFiles int64, isPaused, isOCREnabled *atomic.Bool, filesScanned *atomic.Int64) (<-chan types.RiskTag, <-chan types.FileJob, *sync.WaitGroup, chan types.FileJob) {
	o.channels = NewChannels(o.cfg.PipelineBufferSize)
	processedJobsChan := make(chan types.FileJob, o.cfg.PipelineBufferSize)

	o.dispatcher = dispatcher.New(o.logger, o.channels.Extractor)
	o.dispatcher.Start(ctx)

	var extractorWg, scannerWg sync.WaitGroup

	extractorWg.Add(1)
	// The extractor now needs the OcrJobs channel and the isOCREnabled flag.
	go extractor.StartWorkers(ctx, o.cfg, o.channels.Extractor, o.channels.Scanner, o.channels.OcrJobs, processedJobsChan, isOCREnabled, &extractorWg, o.logger)

	scannerWg.Add(o.cfg.ScannerWorkers)
	scanner.Start(ctx, o.cfg, o.engine, o.channels.Scanner, o.channels.Results, filesScanned, &scannerWg)

	gov := governor.New(o.cfg, o.logger)
	resourcePaused := gov.IsPaused()
	go gov.Start(ctx)
	go o.feedJobsFromDatabase(ctx, isPaused, resourcePaused, totalFiles, filesScanned)

	var processingWg sync.WaitGroup
	processingWg.Add(1)
	go func() {
		defer processingWg.Done()
		extractorWg.Wait()
		close(o.channels.Scanner)
		close(o.channels.OcrJobs) // Close the OcrJobs channel when the extractor is done.
		scannerWg.Wait()
		close(o.channels.Results)
	}()

	return o.channels.Results, processedJobsChan, &processingWg, o.channels.OcrJobs
}

func (o *Orchestrator) GetTotalQueueSize() int {
	if o.dispatcher != nil {
		return o.dispatcher.GetTotalQueueSize()
	}
	return 0
}

// feedJobsFromDatabase now checks the pause flags.
func (o *Orchestrator) feedJobsFromDatabase(ctx context.Context, manualPaused, resourcePaused *atomic.Bool, totalFiles int64, filesScanned *atomic.Int64) {
	defer o.dispatcher.Stop()
	const batchSize = 100
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check both manual and resource-based pause flags
			if manualPaused.Load() || resourcePaused.Load() {
				continue
			}

			if totalFiles > 0 && filesScanned.Load() >= totalFiles {
				o.logger.Info("Feeder: All discovered jobs have been processed.")
				return
			}

			jobsInBatch, err := o.db.GetPendingJobsBatch(ctx, batchSize)
			if err != nil {
				o.logger.Error("Failed to query jobs batch from database", "error", err)
				continue
			}

			if len(jobsInBatch) > 0 {
				for _, job := range jobsInBatch {
					o.dispatcher.Schedule(job)
				}
				if err := o.db.UpdateJobsStatus(ctx, jobsInBatch, "processing"); err != nil {
					o.logger.Error("Feeder: Failed to commit status update transaction", "error", err)
				}
			}
		}
	}
}
