// File: types/types.go
package types

import (
	"io"
	"strings"
	"time"
)

// --- Job Priority ---
type Priority int

const (
	PriorityOCR    Priority = 0 // The absolute lowest priority
	PriorityLow    Priority = 1
	PriorityMedium Priority = 2
	PriorityHigh   Priority = 3
)

// --- Core Data Structures ---

type FileJob struct {
	ID        int64    `json:"id"`
	NodeID    int64    `json:"node_id"` // Link to hierarchy_nodes
	Path      string   `json:"path"`
	Extension string   `json:"extension"`
	Priority  Priority `json:"-"`
}

func (j *FileJob) SetPriority() {
	// Simple text files get high priority
	switch strings.ToLower(j.Extension) {
	case ".txt", ".xml", ".json", ".csv", ".log", ".md", ".html", ".js", ".css", ".go", ".py", ".java", ".c", ".cpp", ".h", ".cs", ".ts", ".yml", ".yaml", ".toml", ".ini", ".cfg", ".sh", ".env":
		j.Priority = PriorityHigh
	// Complex formats get medium priority
	case ".docx", ".xlsx", ".pptx", ".odt":
		j.Priority = PriorityMedium
	// PDF is low priority by default, can be demoted to OCR
	case ".pdf":
		j.Priority = PriorityLow
	// NEW: Image files that require OCR get the lowest priority
	case ".png", ".jpg", ".jpeg", ".tiff", ".bmp", ".gif":
		j.Priority = PriorityOCR
	default:
		j.Priority = PriorityLow
	}
}

type ExtractedContent struct {
	FileJob
	ContentStream io.Reader
	Closer        io.Closer
}

type Match struct {
	FilePath   string `json:"file_path"`
	LineNumber int    `json:"line_number"`
	RuleID     string `json:"rule_id"`
	RuleName   string `json:"rule_name"`
	Value      string `json:"value"` // Deprecated in favor of RiskTag
	StartIndex int    `json:"start_index"`
	EndIndex   int    `json:"end_index"`
}

// Optimization: Use Tags instead of Full Matches for Privacy
type RiskTag struct {
	NodeID    int64   `json:"node_id"`
	RuleID    string  `json:"rule_id"`
	RuleName  string  `json:"rule_name"`
	RiskScore float64 `json:"risk_score"`
	Count     int     `json:"count"`
	// ML Validation Metadata
	ConfidenceScore  float64   `json:"confidence_score"`
	VectorEmbedding  []float64 `json:"vector_embedding"` // Serialized as BLOB/JSON in DB
	ValidationStatus string    `json:"validation_status"`
	PIITypeRefined   string    `json:"pii_type_refined"`
}

type NodeType string

const (
	NodeTypeInfra  NodeType = "infra"
	NodeTypeServer NodeType = "server"
	NodeTypeDrive  NodeType = "drive"
	NodeTypeFolder NodeType = "folder"
	NodeTypeFile   NodeType = "file"
)

type HierarchyNode struct {
	ID       int64    `json:"id"`
	ParentID int64    `json:"parent_id"`
	Name     string   `json:"name"`
	Type     NodeType `json:"type"`
	Path     string   `json:"path"`
}

// --- Frontend Payloads ---

type ProgressUpdate struct {
	FilesScanned int64  `json:"filesScanned"`
	TotalFiles   int64  `json:"totalFiles"`
	Message      string `json:"message"`
}

type ScanCompletePayload struct {
	FilesScanned int64 `json:"filesScanned"`
}

// LogEntry is the structure sent to the frontend.
type LogEntry struct {
	Time    time.Time      `json:"Time"`
	Level   string         `json:"Level"`
	Msg     string         `json:"Msg"`
	Details map[string]any `json:"Details,omitempty"`
}
