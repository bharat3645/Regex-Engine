package governor

import (
	"github.com/bharat3645/compliance-manager/config"
	"github.com/bharat3645/compliance-manager/logger"
	"github.com/bharat3645/compliance-manager/stats"
	"context"
	"sync/atomic"
	"time"
)

// Governor manages the application's resource usage.
type Governor struct {
	cfg       *config.Config
	logger    *logger.AppLogger
	isPaused  *atomic.Bool
	lastState bool
}

// New creates a new Governor.
func New(cfg *config.Config, appLogger *logger.AppLogger) *Governor {
	return &Governor{
		cfg:      cfg,
		logger:   appLogger,
		isPaused: &atomic.Bool{},
	}
}

// IsPaused returns a pointer to the atomic boolean that holds the pause state.
// The pipeline feeders will check this value.
func (g *Governor) IsPaused() *atomic.Bool {
	return g.isPaused
}

// Start runs the main monitoring loop of the Governor.
func (g *Governor) Start(ctx context.Context) {
	g.logger.Info("Starting resource governor...", "max_cpu", g.cfg.MaxCPUPercentage)
	// Check resources more frequently to be responsive.
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			g.logger.Info("Governor shutting down.")
			return
		case <-ticker.C:
			s, err := stats.GetSystemStats(g.logger)
			if err != nil {
				g.logger.Warn("Governor could not get system stats", "error", err)
				continue
			}

			// The core decision logic
			if s.CPUUsage > float64(g.cfg.MaxCPUPercentage) {
				g.isPaused.Store(true)
				if !g.lastState {
					g.logger.Info("Governor: CPU usage high, pausing pipeline.", "cpu_usage", s.CPUUsage)
					g.lastState = true
				}
			} else {
				g.isPaused.Store(false)
				if g.lastState {
					g.logger.Info("Governor: CPU usage normal, resuming pipeline.", "cpu_usage", s.CPUUsage)
					g.lastState = false
				}
			}
		}
	}
}
