// File: main.go
package main

import (
	"github.com/bharat3645/compliance-manager/config"
	"github.com/bharat3645/compliance-manager/core"
	"github.com/bharat3645/compliance-manager/database"
	"github.com/bharat3645/compliance-manager/logger"
	"github.com/bharat3645/compliance-manager/stats"
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

const preliminaryLogFile = "startup_errors.log"

// WailsEventEmitter is a struct that implements the EventEmitter interface
// using the Wails runtime.
type WailsEventEmitter struct {
	ctx context.Context
}

// Emit sends an event to the frontend via the Wails runtime.
func (e *WailsEventEmitter) Emit(eventName string, payload ...interface{}) {
	wailsruntime.EventsEmit(e.ctx, eventName, payload...)
}

// App struct holds the application's state and methods.
type App struct {
	ctx         context.Context
	logger      *logger.AppLogger
	db          *database.Manager
	scanManager *core.ScanManager
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// preliminaryLog is a failsafe logger for critical startup errors.
func preliminaryLog(message string) {
	// Implementation remains the same...
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	cfg, err := config.Load()
	if err != nil {
		preliminaryLog(fmt.Sprintf("FATAL: Could not load config.json: %v", err))
		a.logger, _ = logger.New(ctx, "main", "INFO", "")
	} else {
		var logErr error
		a.logger, logErr = logger.New(ctx, "main", cfg.LogLevel, cfg.LogFile)
		if logErr != nil {
			preliminaryLog(fmt.Sprintf("ERROR: Failed to initialize file logger: %v", logErr))
			// Fallback to stdout logger
			a.logger, _ = logger.New(ctx, "main", cfg.LogLevel, "")
		}
	}

	dbManager, err := database.NewManager()
	if err != nil {
		a.logger.Error("FATAL: Could not initialize database", "error", err)
		wailsruntime.Quit(ctx)
		return
	}
	a.db = dbManager
	a.logger.Info("Database manager initialized successfully.")

	// Create the Wails event emitter and pass it to the ScanManager.
	eventEmitter := &WailsEventEmitter{ctx: a.ctx}
	a.scanManager = core.NewScanManager(a.logger, a.db, eventEmitter)
	a.logger.Info("Scan manager initialized successfully.")
	a.logger.Info("Application startup complete.")
}

// shutdown is called when the app is about to close.
func (a *App) shutdown(ctx context.Context) {
	if a.db != nil {
		a.db.Close()
	}
	if a.logger != nil {
		a.logger.Close()
	}
}

// --- Frontend Bindings ---

// TreeNode represents a node in the directory tree for the UI.
type TreeNode struct {
	ID        int64            `json:"id"`
	ParentID  int64            `json:"parent_id"`
	Name      string           `json:"name"`
	Type      string           `json:"type"` // "folder", "file", "drive"
	Children  bool             `json:"children"` // True if it has children (for lazy loading)
	RiskScore float64          `json:"risk_score"`
	Edges     []string         `json:"edges"` // e.g., ["CopyOf: /path/to/other"]
}

// GetTreeNodes returns children of a given parent node.
// parentID = 0 returns Root nodes (Infra/Machine).
func (a *App) GetTreeNodes(parentID int64) []TreeNode {
	// 1. Get nodes
	query := `
		SELECT id, parent_id, name, type, path
		FROM hierarchy_nodes 
		WHERE parent_id = ?
	`
	rows, err := a.db.DB().Query(query, parentID)
	if err != nil {
		a.logger.Error("Failed to query tree nodes", "parent", parentID, "error", err)
		return []TreeNode{}
	}
	defer rows.Close()

	var nodes []TreeNode
	for rows.Next() {
		var n TreeNode
		var path string
		if err := rows.Scan(&n.ID, &n.ParentID, &n.Name, &n.Type, &path); err != nil {
			continue
		}
		
		// 2. Check for children (Optimized: Sub-query exists check?)
		if n.Type != "file" {
			n.Children = true 
		}
		
		// 3. Get Risk Score (Sum of PII tags on this node)
		riskQuery := "SELECT COALESCE(SUM(risk_score), 0) FROM pii_tags WHERE node_id = ?"
		_ = a.db.DB().QueryRow(riskQuery, n.ID).Scan(&n.RiskScore)

		// 4. Get Lineage Edges (Source)
		// We want to know if THIS node is a "CopyOf" something else.
		// Or if something else is a "CopyOf" this node.
		edgeQuery := `
			SELECT r.type, t.path 
			FROM node_relations r
			JOIN hierarchy_nodes t ON r.target_node_id = t.id
			WHERE r.source_node_id = ?
		`
		edgeRows, err := a.db.DB().Query(edgeQuery, n.ID)
		if err == nil {
			for edgeRows.Next() {
				var rType, tPath string
				if err := edgeRows.Scan(&rType, &tPath); err == nil {
					n.Edges = append(n.Edges, fmt.Sprintf("%s -> %s", rType, tPath))
				}
			}
			edgeRows.Close()
		}
		
		nodes = append(nodes, n)
	}
	return nodes
}

// SetOCREnabled is exposed to the frontend to toggle OCR scanning.
func (a *App) SetOCREnabled(enabled bool) {
	if a.scanManager != nil {
		a.scanManager.SetOCREnabled(enabled)
		if enabled {
			a.logger.Info("OCR scanning has been ENABLED by the user.")
		} else {
			a.logger.Info("OCR scanning has been DISABLED by the user.")
		}
	}
}

// GetSystemStats is exposed to the frontend.
func (a *App) GetSystemStats() (*stats.SystemStats, error) {
	s, err := stats.GetSystemStats(a.logger)
	if err != nil {
		a.logger.Warn("Failed to get system stats", "error", err)
		return nil, err
	}
	if a.scanManager != nil {
		s.QueueDepth = a.scanManager.GetQueueDepth()
	}
	return s, nil
}

// GetConfig is exposed to the frontend.
func (a *App) GetConfig() (*config.Config, error) {
	return config.Load()
}

// SaveConfig is exposed to the frontend.
func (a *App) SaveConfig(newConfig *config.Config) error {
	return config.Save(newConfig)
}

// StartScan is exposed to the frontend.
func (a *App) StartScan() {
	if err := a.preflightChecks(); err != nil {
		// Use the ScanManager's error handler which is now decoupled.
		a.scanManager.HandleError(err.Error())
		return
	}
	a.scanManager.Start()
}

// StopScan is exposed to the frontend.
func (a *App) StopScan() {
	a.scanManager.Stop()
}

// PauseScan is exposed to the frontend.
func (a *App) PauseScan() {
	if a.scanManager != nil {
		a.scanManager.PauseScan()
	}
}

// ResumeScan is exposed to the frontend.
func (a *App) ResumeScan() {
	if a.scanManager != nil {
		a.scanManager.ResumeScan()
	}
}

// preflightChecks verifies that essential files and directories exist.
func (a *App) preflightChecks() error {
	a.logger.Info("Performing pre-flight dependency checks...")
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config for pre-flight checks: %w", err)
	}

	rulesPath, err := config.GetPath(cfg.RulesDir)
	if err != nil {
		return fmt.Errorf("could not resolve rules directory path from config value '%s': %w", cfg.RulesDir, err)
	}
	if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
		return fmt.Errorf("rules directory not found. Application expected it at the following location: %s", rulesPath)
	}

	a.logger.Info("All dependency checks passed.")
	return nil
}

func main() {
	exePath, err := os.Executable()
	if err != nil {
		preliminaryLog(fmt.Sprintf("FATAL: Could not get executable path: %v", err))
		os.Exit(1)
	}
	exeDir := filepath.Dir(exePath)
	if err := os.Chdir(exeDir); err != nil {
		preliminaryLog(fmt.Sprintf("FATAL: Could not change to executable directory: %v", err))
		os.Exit(1)
	}

	app := NewApp()
	err = wails.Run(&options.App{
		Title:  "Compliance Manager",
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 26, G: 26, B: 26, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		preliminaryLog(fmt.Sprintf("FATAL: Wails failed to start: %v", err))
	}
}
