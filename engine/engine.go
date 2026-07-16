// File: engine/engine.go
package engine

import (
	"Regex/logger"
	"Regex/rules"
	"Regex/types"
	"fmt"

	"github.com/grafana/regexp" // USE PURE GO, RELIABLE ALTERNATIVE
)

// Engine is the core regex scanner.
type Engine struct {
	rules         []rules.Rule
	compiledRegex map[string]*regexp.Regexp // Use grafana/regexp.Regexp
	Logger        *logger.AppLogger
}

// NewEngine initializes a new regex engine with the pure Go Grafana/Regexp library.
func NewEngine(loadedRules []rules.Rule, appLogger *logger.AppLogger) (*Engine, error) {
	appLogger.Info("Initializing Grafana/Regexp engine for secure matching", "total_rules", len(loadedRules))

	engine := &Engine{
		rules:         make([]rules.Rule, 0, len(loadedRules)),
		compiledRegex: make(map[string]*regexp.Regexp, len(loadedRules)),
		Logger:        appLogger,
	}

	successful := 0
	for _, rule := range loadedRules {
		// The compile function is the same as the standard library
		re, err := regexp.Compile("(?i)" + rule.Pattern)
		if err != nil {
			engine.Logger.Error("Grafana/Regexp compile failed, skipping rule",
				"rule_id", rule.ID,
				"rule_name", rule.Name,
				"error", err)
			continue
		}
		engine.compiledRegex[rule.ID] = re
		engine.rules = append(engine.rules, rule)
		successful++
	}

	if successful == 0 && len(loadedRules) > 0 {
		return nil, fmt.Errorf("all provided rules failed to compile with Grafana/Regexp")
	}

	engine.Logger.Info("Grafana/Regexp engine ready",
		"compiled_rules", successful,
		"skipped_rules", len(loadedRules)-successful)

	return engine, nil
}

// Scan runs all rules against the given text.
func (e *Engine) Scan(text string) []types.Match {
	matches := make([]types.Match, 0, 4)

	for _, rule := range e.rules {
		re := e.compiledRegex[rule.ID]
		if re == nil {
			continue
		}

		// Find indices of all matches
		foundIndices := re.FindAllStringIndex(text, -1)
		for _, loc := range foundIndices {
			start, end := loc[0], loc[1]
			val := text[start:end]

			matches = append(matches, types.Match{
				RuleID:     rule.ID,
				RuleName:   rule.Name,
				Value:      val,
				StartIndex: start,
				EndIndex:   end,
			})
		}
	}

	return matches
}
