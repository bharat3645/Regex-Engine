// File: config/config.go
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

// Config holds all runtime settings for the application.
type Config struct {
	RootDir             string `json:"root_dir"`
	RulesDir            string `json:"rules_dir"`
	OutputFile          string `json:"output_file"`
	LogFile             string `json:"log_file"`
	ScannerWorkers      int    `json:"scanner_workers"`   // RENAMED for clarity
	ExtractorWorkers    int    `json:"extractor_workers"` // RENAMED for clarity
	LogLevel            string `json:"log_level"`
	MaxCPUPercentage    int    `json:"max_cpu_percentage"`
	PipelineBufferSize  int    `json:"pipeline_buffer_size"`
	ScannerBufferSizeMB int    `json:"scanner_buffer_size_mb"`
	DiscoveryBatchSize  int    `json:"discovery_batch_size"`

	// Connector Settings
	DataSourceType   string            `json:"data_source_type"`   // e.g. "local_fs", "s3"
	DataSourceConfig map[string]string `json:"data_source_config"` // Generic config for connector

	// ML Settings
	MLServerURL string `json:"ml_server_url"` // URL of the Python Hexa-Core Engine

	GCTriggerMB         int64  `json:"gc_trigger_mb"`
	MaxFileSizeMB       int64  `json:"max_file_size_mb"` // NEW: File size limit
	OCRPort             int    `json:"ocr_port"`         // NEW: Configurable OCR port
}

// getConfigPath determines the absolute path to the config.json file.
func getConfigPath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exePath), "config.json"), nil
}

// GetPath determines the absolute path to a file relative to the executable.
func GetPath(relativePath string) (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exePath), relativePath), nil
}

// Load reads and parses the configuration file.
func Load() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Set sane defaults for any missing values
	if cfg.LogFile == "" {
		cfg.LogFile = "scan.log"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "INFO"
	}
	if cfg.ScannerWorkers <= 0 {
		cfg.ScannerWorkers = runtime.NumCPU() / 2
		if cfg.ScannerWorkers == 0 {
			cfg.ScannerWorkers = 1
		}
	}
	if cfg.ExtractorWorkers <= 0 {
		cfg.ExtractorWorkers = 4
	}
	if cfg.MaxCPUPercentage <= 0 || cfg.MaxCPUPercentage > 100 {
		cfg.MaxCPUPercentage = 80
	}
	if cfg.PipelineBufferSize <= 0 {
		cfg.PipelineBufferSize = 100
	}
	if cfg.ScannerBufferSizeMB <= 0 {
		cfg.ScannerBufferSizeMB = 10
	}
	if cfg.DiscoveryBatchSize <= 0 {
		cfg.DiscoveryBatchSize = 500
	}
	if cfg.DataSourceType == "" {
		cfg.DataSourceType = "local_fs" // Default to local filesystem
	}
	if cfg.MLServerURL == "" {
		cfg.MLServerURL = "http://localhost:8000"
	}
	if cfg.GCTriggerMB <= 0 {
		cfg.GCTriggerMB = 1024
	}
	if cfg.MaxFileSizeMB <= 0 {
		cfg.MaxFileSizeMB = 500 // Default file size limit of 500MB
	}
	if cfg.OCRPort <= 0 {
		cfg.OCRPort = 50051 // Default gRPC port
	}
	return &cfg, nil
}

// Save writes the provided config struct back to the specified JSON file.
func Save(cfg *Config) error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}
