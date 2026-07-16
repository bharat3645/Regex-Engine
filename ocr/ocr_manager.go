// File: ocr/ocr_manager.go
package ocr

import (
	"Regex/logger"
	"Regex/types"
	"context"
	"fmt"
	"log" // Add this import for the standard log package
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// OCRManager handles the lifecycle of the Python OCR worker and the gRPC client.
type OCRManager struct {
	logger      *logger.AppLogger
	cmd         *exec.Cmd
	gRPCClient  OCRServiceClient
	conn        *grpc.ClientConn
	OcrJobsChan chan types.FileJob
	outputChan  chan<- types.ExtractedContent
	wg          sync.WaitGroup
	port        int
}

// NewOCRManager creates a new manager for the OCR process.
func NewOCRManager(appLogger *logger.AppLogger, outputChan chan<- types.ExtractedContent, port int) *OCRManager {
	return &OCRManager{
		logger:      appLogger,
		OcrJobsChan: make(chan types.FileJob, 100), // Buffer for OCR jobs
		outputChan:  outputChan,
		port:        port,
	}
}

// Start launches the Python worker and establishes the gRPC connection.
func (m *OCRManager) Start(ctx context.Context) error {
	m.logger.Info("Starting Python OCR worker...", "port", m.port)

	// Launch the bundled Python executable with port argument
	m.cmd = exec.Command("./ocr_binaries/ocr_worker.exe", fmt.Sprintf("--port=%d", m.port))

	// Redirect the Python subprocess's stdout and stderr to the Go logger.
	m.cmd.Stdout = log.Writer()
	m.cmd.Stderr = log.Writer()

	// Hide the terminal window on Windows
	if runtime.GOOS == "windows" {
		m.cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow: true,
		}
	}

	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start python ocr worker: %w", err)
	}
	m.logger.Info("Python OCR worker process started", "pid", m.cmd.Process.Pid)

	var err error
	for i := 0; i < 5; i++ {
		m.conn, err = grpc.Dial(fmt.Sprintf("localhost:%d", m.port), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err == nil {
			break
		}
		m.logger.Warn("Failed to connect to gRPC server, retrying...", "attempt", i+1)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return fmt.Errorf("could not connect to ocr grpc server after retries: %w", err)
	}

	m.gRPCClient = NewOCRServiceClient(m.conn)
	m.logger.Info("Successfully connected to OCR gRPC server.")

	m.wg.Add(1)
	go m.processJobs(ctx)

	return nil
}

// processJobs reads from the jobs channel and sends requests to the Python worker.
func (m *OCRManager) processJobs(ctx context.Context) {
	defer m.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-m.OcrJobsChan:
			if !ok {
				return
			}

			job.Priority = types.PriorityOCR

			res, err := m.gRPCClient.ProcessPdf(ctx, &OCRRequest{FilePath: job.Path})
			if err != nil {
				m.logger.Error("OCR processing failed for file", "path", job.Path, "error", err)
				continue
			}

			if res.Error != "" {
				m.logger.Warn("Python worker returned error for file", "path", job.Path, "error", res.Error)
				continue
			}

			content := types.ExtractedContent{
				FileJob:       job,
				ContentStream: strings.NewReader(res.ExtractedText),
				Closer:        nil,
			}
			m.outputChan <- content
		}
	}
}

// Stop terminates the gRPC connection and the Python subprocess.
func (m *OCRManager) Stop() {
	m.logger.Info("Stopping OCR manager...")

	// DO NOT CLOSE THE INPUT CHANNEL HERE. The sender is responsible for that.
	// This was the source of the race condition.

	m.wg.Wait() // Wait for the job processing goroutine to finish reading the channel.

	if m.conn != nil {
		m.conn.Close()
		m.logger.Info("gRPC connection closed.")
	}
	if m.cmd != nil && m.cmd.Process != nil {
		var err error
		if runtime.GOOS == "windows" {
			err = exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", m.cmd.Process.Pid)).Run()
		} else {
			// Graceful shutdown on non-Windows
			if err := m.cmd.Process.Signal(syscall.SIGTERM); err != nil {
				m.logger.Warn("Failed to send SIGTERM to OCR worker", "error", err)
			}
			
			// Wait for a short duration to allow graceful exit
			done := make(chan error, 1)
			go func() {
				done <- m.cmd.Wait()
			}()
			
			select {
			case <-done:
				m.logger.Info("OCR worker exited gracefully")
			case <-time.After(2 * time.Second):
				m.logger.Warn("OCR worker did not exit, forcing KILL")
				err = m.cmd.Process.Kill()
			}
		}

		if err != nil {
			m.logger.Error("Failed to stop Python OCR worker process", "pid", m.cmd.Process.Pid, "error", err)
		} else {
			m.logger.Info("Python OCR worker process stopped.")
		}
	}
}
