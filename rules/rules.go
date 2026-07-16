package rules

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// Rule defines the structure for a single compliance rule.
type Rule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Pattern     string `json:"pattern"`
}

// LoadFromFolder now correctly handles JSON files that contain an array of rules.
func LoadFromFolder(folderPath string) ([]Rule, error) {
	slog.Info("Starting to load rules", "folder", folderPath)
	files, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read rules directory %s: %w", folderPath, err)
	}

	var allRules []Rule
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(folderPath, file.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			slog.Warn("Skipping rule file due to read error", "path", filePath, "error", err)
			continue
		}

		var rulesInFile []Rule
		if err := json.Unmarshal(content, &rulesInFile); err != nil {
			slog.Warn("Skipping rule file due to parsing error", "path", filePath, "error", err)
			continue
		}
		allRules = append(allRules, rulesInFile...)
	}

	if len(allRules) == 0 {
		return nil, fmt.Errorf("no valid rules found in %s", folderPath)
	}

	slog.Info("Successfully loaded rules", "count", len(allRules))
	return allRules, nil
}
