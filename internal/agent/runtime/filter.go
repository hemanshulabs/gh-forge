package runtime

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dlvhdr/gh-dash/v4/internal/agent"
)

// FilterFindings drops findings whose confidence is below minConfidence.
func FilterFindings(findings []agent.Finding, minConfidence float64) []agent.Finding {
	if minConfidence <= 0.0 {
		return findings
	}

	filtered := make([]agent.Finding, 0, len(findings))
	for _, f := range findings {
		if f.Confidence >= minConfidence {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

// DeduplicateFindings removes duplicate findings on the same file, line, and similar explanation.
func DeduplicateFindings(findings []agent.Finding) []agent.Finding {
	seen := make(map[string]bool)
	unique := make([]agent.Finding, 0, len(findings))

	for _, f := range findings {
		key := fmt.Sprintf("%s:%d:%s", strings.ToLower(f.File), f.Line, strings.ToLower(strings.TrimSpace(f.Explanation)))
		if !seen[key] {
			seen[key] = true
			unique = append(unique, f)
		}
	}
	return unique
}

var severityWeights = map[agent.Severity]int{
	agent.SeverityCritical: 5,
	agent.SeverityHigh:     4,
	agent.SeverityMedium:   3,
	agent.SeverityLow:      2,
	agent.SeverityInfo:     1,
}

// SortFindings sorts findings by severity (highest first) and then by file and line.
func SortFindings(findings []agent.Finding) []agent.Finding {
	res := make([]agent.Finding, len(findings))
	copy(res, findings)

	sort.SliceStable(res, func(i, j int) bool {
		wI := severityWeights[res[i].Severity]
		wJ := severityWeights[res[j].Severity]
		if wI != wJ {
			return wI > wJ
		}
		if res[i].File != res[j].File {
			return res[i].File < res[j].File
		}
		return res[i].Line < res[j].Line
	})

	return res
}
