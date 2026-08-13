package sorter

import (
	"github.com/bharat3645/compliance-manager/logger"
	"github.com/bharat3645/compliance-manager/types"
	"context"
	"sync"
)

// Start sorts incoming FileJobs into typed channels for different extractors.
func Start(ctx context.Context, jobs <-chan types.FileJob, textJobs chan<- types.FileJob, pythonJobs chan<- types.FileJob, wg *sync.WaitGroup, appLogger *logger.AppLogger) {
	defer wg.Done()
	appLogger.Info("Triage/Sorter stage started.")

	for {
		select {
		case <-ctx.Done():
			appLogger.Info("Sorter stage received stop signal.")
			// When the context is cancelled, close downstream channels and exit.
			close(textJobs)
			close(pythonJobs)
			return
		case job, ok := <-jobs:
			if !ok {
				// When the input channel is closed, close downstream channels and exit.
				close(textJobs)
				close(pythonJobs)
				return
			}
			switch job.Extension {
			case ".txt", ".xml", ".json", ".csv", ".log", ".md", ".html", ".js", ".css", ".go", ".py", ".java", ".c", ".cpp", ".h", ".cs", ".ts", ".yml", ".yaml", ".toml", ".ini", ".cfg":
				textJobs <- job
			case ".pdf", ".docx", ".xlsx", ".pptx":
				pythonJobs <- job
			default:
				appLogger.Debug("Sorter treating unknown file type as text", "path", job.Path, "type", job.Extension)
				textJobs <- job
			}
		}
	}
}
