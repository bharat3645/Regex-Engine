package client

import (
	"github.com/bharat3645/compliance-manager/logger"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// MLClient handles communication with the Python Hexa-Core Engine
type MLClient struct {
	BaseURL    string
	HTTPClient *http.Client
	Logger     *logger.AppLogger
}

// NewMLClient creates a new client
func NewMLClient(baseURL string, logger *logger.AppLogger) *MLClient {
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}
	return &MLClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		Logger: logger,
	}
}

// PIIContext matches python 'PIIContext' schema
type PIIContext struct {
	TextSegment string                 `json:"text_segment"`
	StartIndex  int                    `json:"start_index"`
	EndIndex    int                    `json:"end_index"`
	FilePath    string                 `json:"file_path"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// ScanRequest matches python 'ScanRequest' schema
type ScanRequest struct {
	RequestID  string       `json:"request_id"`
	Candidates []PIIContext `json:"candidates"`
}

// ScanResponse matches python 'ScanResponse' schema
type ScanResponse struct {
	RequestID      string           `json:"request_id"`
	ProcessedCount int              `json:"processed_count"`
	Results        []ValidatedMatch `json:"results"`
}

// ValidatedMatch matches python 'ValidatedMatch' schema
type ValidatedMatch struct {
	OriginalText    string        `json:"original_text"`
	PIIType         string        `json:"pii_type"`
	ConfidenceScore float64       `json:"confidence_score"`
	VectorEmbedding []float64     `json:"vector_embedding"`
	IsValid         bool          `json:"is_valid"`
	LayerBreakdown  []LayerResult `json:"layer_breakdown"`
}

type LayerResult struct {
	LayerName string                 `json:"layer_name"`
	Score     float64                `json:"score"`
	Decision  string                 `json:"decision"`
	Details   map[string]interface{} `json:"details"`
}

// HealthCheck verifies if the ML server is reachable.
func (c *MLClient) HealthCheck(ctx context.Context) (bool, error) {
	url := fmt.Sprintf("%s/health", c.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	return false, fmt.Errorf("bad status: %d", resp.StatusCode)
}

// ScanCandidates sends a batch of candidates to the ML Server with retry logic.
func (c *MLClient) ScanCandidates(ctx context.Context, candidates []PIIContext) ([]ValidatedMatch, error) {
	if len(candidates) == 0 {
		return []ValidatedMatch{}, nil
	}

	reqBody := ScanRequest{
		RequestID:  fmt.Sprintf("req-%d", time.Now().UnixNano()),
		Candidates: candidates,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/scan", c.BaseURL)

	// Retry Configuration
	maxRetries := 3
	backoff := 500 * time.Millisecond

	var resp *http.Response
	var reqErr error

	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, reqErr = c.HTTPClient.Do(req)
		if reqErr == nil && resp.StatusCode == http.StatusOK {
			// Success
			break
		}

		if resp != nil {
			resp.Body.Close()
		}

		// If error or bad status, wait and retry
		if c.Logger != nil {
			c.Logger.Warn("ML Server connection failed, retrying...", "attempt", i+1, "error", reqErr)
		}
		time.Sleep(backoff)
		backoff *= 2
	}

	if reqErr != nil {
		return nil, fmt.Errorf("ml server request failed after retries: %w", reqErr)
	}
	if resp == nil {
		return nil, fmt.Errorf("ml server request failed (empty response)")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ml server returned incorrect status: %d", resp.StatusCode)
	}

	var scanResp ScanResponse
	if err := json.NewDecoder(resp.Body).Decode(&scanResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return scanResp.Results, nil
}
