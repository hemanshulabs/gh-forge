package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Severity levels for agent findings.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

// IsValid checks if the severity string is a recognized level.
func (s Severity) IsValid() bool {
	switch strings.ToLower(string(s)) {
	case string(SeverityCritical), string(SeverityHigh), string(SeverityMedium), string(SeverityLow), string(SeverityInfo):
		return true
	default:
		return false
	}
}

// Finding represents an individual finding reported by a specialized agent.
type Finding struct {
	Severity     Severity `json:"severity"`
	Confidence   float64  `json:"confidence"`
	File         string   `json:"file"`
	Line         int      `json:"line"`
	Explanation  string   `json:"explanation"`
	SuggestedFix string   `json:"suggested_fix,omitempty"`
}

// AgentResult represents the complete structured output from an agent execution.
type AgentResult struct {
	Role      string    `json:"role"`
	Summary   string    `json:"summary"`
	Findings  []Finding `json:"findings"`
	RawResult string    `json:"raw_result,omitempty"`
}

// Validate checks that findings have valid severity, confidence in [0.0, 1.0], and line >= 0.
func (r *AgentResult) Validate() error {
	if r.Summary == "" && len(r.Findings) == 0 {
		return fmt.Errorf("agent result has neither summary nor findings")
	}
	for i, f := range r.Findings {
		if !f.Severity.IsValid() {
			return fmt.Errorf("finding #%d has invalid severity: %q", i+1, f.Severity)
		}
		if f.Confidence < 0.0 || f.Confidence > 1.0 {
			return fmt.Errorf("finding #%d confidence must be in range [0.0, 1.0], got: %.2f", i+1, f.Confidence)
		}
		if f.Line < 0 {
			return fmt.Errorf("finding #%d line number cannot be negative, got: %d", i+1, f.Line)
		}
		if strings.TrimSpace(f.Explanation) == "" {
			return fmt.Errorf("finding #%d is missing an explanation", i+1)
		}
	}
	return nil
}

// ParseAgentResult parses and validates a JSON string into AgentResult.
func ParseAgentResult(jsonStr string) (*AgentResult, error) {
	// Trim any markdown code blocks if the model wrapped the JSON in ```json ... ```
	cleaned := strings.TrimSpace(jsonStr)
	if strings.HasPrefix(cleaned, "```") {
		lines := strings.Split(cleaned, "\n")
		if len(lines) >= 2 {
			// strip first line (e.g. ```json)
			lines = lines[1:]
			// strip last line if ```
			if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
				lines = lines[:len(lines)-1]
			}
			cleaned = strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}

	var res AgentResult
	if err := json.Unmarshal([]byte(cleaned), &res); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %w", err)
	}

	// Normalize severity to lowercase
	for i := range res.Findings {
		res.Findings[i].Severity = Severity(strings.ToLower(string(res.Findings[i].Severity)))
	}

	if err := res.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	return &res, nil
}

// JSONSchemaSpecification returns the fixed JSON schema expected from all agent completions.
const JSONSchemaSpecification = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "AgentResult",
  "type": "object",
  "required": ["role", "summary", "findings"],
  "properties": {
    "role": {
      "type": "string",
      "description": "Role of the agent (Reviewer, Triage, Sentinel)"
    },
    "summary": {
      "type": "string",
      "description": "High-level summary of the analysis"
    },
    "findings": {
      "type": "array",
      "description": "List of distinct findings",
      "items": {
        "type": "object",
        "required": ["severity", "confidence", "file", "line", "explanation"],
        "properties": {
          "severity": {
            "type": "string",
            "enum": ["critical", "high", "medium", "low", "info"],
            "description": "Severity level of the issue"
          },
          "confidence": {
            "type": "number",
            "minimum": 0.0,
            "maximum": 1.0,
            "description": "Confidence score from 0.0 to 1.0"
          },
          "file": {
            "type": "string",
            "description": "Relative file path of the finding, or empty if general"
          },
          "line": {
            "type": "integer",
            "minimum": 0,
            "description": "Line number (1-based, or 0 if file-level)"
          },
          "explanation": {
            "type": "string",
            "description": "Detailed explanation of the issue"
          },
          "suggested_fix": {
            "type": "string",
            "description": "Actionable suggested fix, diff snippet, or remediation"
          }
        }
      }
    }
  }
}`
