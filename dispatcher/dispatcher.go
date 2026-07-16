// File: dispatcher/dispatcher.go
package dispatcher

import (
	"Regex/logger"
	"Regex/types"
	"container/heap"
	"context"
	"sync"
	"time"
)

// JobDispatcher manages multiple priority queues and distributes jobs.
type JobDispatcher struct {
	logger     *logger.AppLogger
	mainQueue  *PriorityQueue
	outputChan chan<- types.FileJob
	stopChan   chan struct{}
	wg         sync.WaitGroup
	mu         sync.Mutex
}

// New creates a new JobDispatcher.
func New(appLogger *logger.AppLogger, outputChan chan<- types.FileJob) *JobDispatcher {
	// FIX: NewPriorityQueue already returns a pointer, so we don't use the address-of operator (&).
	pq := NewPriorityQueue()
	heap.Init(pq) // Pass the pointer directly.
	return &JobDispatcher{
		logger:     appLogger,
		mainQueue:  pq, // Assign the pointer directly.
		outputChan: outputChan,
		stopChan:   make(chan struct{}),
	}
}

// Start begins the dispatching loop.
func (d *JobDispatcher) Start(ctx context.Context) {
	d.wg.Add(1)
	go d.dispatchLoop(ctx)
	d.logger.Info("Job Dispatcher started.")
}

// dispatchLoop is the main logic for pulling from queues and sending to workers.
func (d *JobDispatcher) dispatchLoop(ctx context.Context) {
	defer d.wg.Done()
	ticker := time.NewTicker(50 * time.Millisecond) // Check for jobs periodically
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopChan:
			// Drain the final jobs
			d.mu.Lock()
			for d.mainQueue.Len() > 0 {
				job := heap.Pop(d.mainQueue).(*types.FileJob)
				d.outputChan <- *job
			}
			d.mu.Unlock()
			return
		case <-ticker.C:
			d.mu.Lock()
			if d.mainQueue.Len() > 0 {
				job := heap.Pop(d.mainQueue).(*types.FileJob)
				// Use a non-blocking send to avoid getting stuck if the extractor channel is full.
				// The job will be retried on the next tick if the channel is busy.
				select {
				case d.outputChan <- *job:
					// Job sent successfully
				default:
					// Channel was full, push the job back onto the heap to be tried again.
					heap.Push(d.mainQueue, job)
				}
			}
			d.mu.Unlock()
		}
	}
}

// Schedule adds a new job to the appropriate priority queue.
func (d *JobDispatcher) Schedule(job types.FileJob) {
	job.SetPriority() // Assign priority based on file type
	d.mu.Lock()
	heap.Push(d.mainQueue, &job)
	d.mu.Unlock()
}

// Stop gracefully shuts down the dispatcher.
func (d *JobDispatcher) Stop() {
	close(d.stopChan)
	d.wg.Wait()
	close(d.outputChan)
	d.logger.Info("Job Dispatcher stopped and channels closed.")
}

// GetTotalQueueSize returns the total number of jobs waiting in all queues.
func (d *JobDispatcher) GetTotalQueueSize() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.mainQueue.Len()
}
