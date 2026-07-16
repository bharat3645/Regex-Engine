// File: pipeline/pipeline.go
package pipeline

import "Regex/types"

// Channels holds all the channels used in the pipeline.
type Channels struct {
	Extractor chan types.FileJob
	Scanner   chan types.ExtractedContent
	Results   chan types.RiskTag // UPDATED: Privacy-first RiskTag instead of Match values
	OcrJobs   chan types.FileJob // NEW: Channel for jobs that need OCR
}

// NewChannels is a helper function that initializes all the channels required.
func NewChannels(bufferSize int) *Channels {
	return &Channels{
		Extractor: make(chan types.FileJob, bufferSize),
		Scanner:   make(chan types.ExtractedContent, bufferSize),
		Results:   make(chan types.RiskTag, bufferSize),
		OcrJobs:   make(chan types.FileJob, bufferSize), // NEW: Initialize the OCR channel
	}
}
