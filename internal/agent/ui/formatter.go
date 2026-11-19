package ui

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/hemanshulabs/gh-forge/v4/internal/agent"
	"github.com/hemanshulabs/gh-forge/v4/internal/tui/constants"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#02F9FB")).
			MarginBottom(1)

	summaryBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#42A0FA")).
			Padding(0, 1).
			MarginBottom(1)

	fileLineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#42A0FA")).
			Bold(true)

	confidenceStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8BE9FD")).
			Faint(true)

	suggestedFixStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("#6272A4")).
				Background(lipgloss.Color("#1E1E2C")).
				Foreground(lipgloss.Color("#50FA7B")).
				Padding(0, 1).
				MarginTop(1)

	severityBadges = map[agent.Severity]lipgloss.Style{
		agent.SeverityCritical: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#E06C75")).
			Padding(0, 1),
		agent.SeverityHigh: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#D19A66")).
			Padding(0, 1),
		agent.SeverityMedium: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#282C34")).
			Background(lipgloss.Color("#E5C07B")).
			Padding(0, 1),
		agent.SeverityLow: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#282C34")).
			Background(lipgloss.Color("#61AFEF")).
			Padding(0, 1),
		agent.SeverityInfo: lipgloss.NewStyle().
			Faint(true).
			Foreground(lipgloss.Color("#ABB2BF")),
	}
)

// RenderResult renders an AgentResult according to the specified format ("pretty", "json", "markdown").
func RenderResult(result *agent.AgentResult, format string) (string, error) {
	switch strings.ToLower(format) {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil

	case "markdown", "md":
		return renderMarkdown(result), nil

	default:
		return RenderPretty(result), nil
	}
}

// RenderPretty renders an AgentResult using Lipgloss styles consistent with Forge.
func RenderPretty(result *agent.AgentResult) string {
	var b strings.Builder

	header := fmt.Sprintf("%s FORGE AI %s", constants.AgentIcon, strings.ToUpper(result.Role))
	b.WriteString(titleStyle.Render(header))
	b.WriteString("\n")

	if result.Summary != "" {
		summaryContent := fmt.Sprintf("Summary: %s", result.Summary)
		b.WriteString(summaryBoxStyle.Render(summaryContent))
		b.WriteString("\n\n")
	}
	for _, warning := range result.Warnings {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B")).Render("Warning: " + warning))
		b.WriteString("\n")
	}
	if len(result.Warnings) > 0 {
		b.WriteString("\n")
	}

	if len(result.Findings) == 0 {
		message := "No issues identified by agent."
		if len(result.Warnings) > 0 {
			message = "No findings in the completed analysis."
		}
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B")).Render(fmt.Sprintf("  %s %s", constants.SuccessIcon, message)))
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString(fmt.Sprintf("Found %d finding(s):\n\n", len(result.Findings)))

	for i, f := range result.Findings {
		badge, ok := severityBadges[f.Severity]
		if !ok {
			badge = severityBadges[agent.SeverityInfo]
		}
		badgeRendered := badge.Render(strings.ToUpper(string(f.Severity)))

		loc := "General"
		if f.File != "" {
			if f.Line > 0 {
				loc = fmt.Sprintf("%s:%d", f.File, f.Line)
			} else {
				loc = f.File
			}
		}

		conf := fmt.Sprintf("(confidence: %.0f%%)", f.Confidence*100)

		headerLine := fmt.Sprintf(" %2d. %s  %s  %s", i+1, badgeRendered, fileLineStyle.Render(loc), confidenceStyle.Render(conf))
		b.WriteString(headerLine)
		b.WriteString("\n")

		// Explanation indented
		expStyle := lipgloss.NewStyle().PaddingLeft(6).Foreground(lipgloss.Color("#F8F8F2"))
		b.WriteString(expStyle.Render(f.Explanation))
		b.WriteString("\n")

		if strings.TrimSpace(f.SuggestedFix) != "" {
			fixLabel := lipgloss.NewStyle().PaddingLeft(6).Foreground(lipgloss.Color("#BD93F9")).Bold(true).Render("Suggested Fix:")
			b.WriteString(fixLabel)
			b.WriteString("\n")
			fixBox := lipgloss.NewStyle().MarginLeft(6).Render(suggestedFixStyle.Render(f.SuggestedFix))
			b.WriteString(fixBox)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

func renderMarkdown(result *agent.AgentResult) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## Forge AI %s\n\n", result.Role))
	b.WriteString(fmt.Sprintf("> **Summary:** %s\n\n", result.Summary))
	for _, warning := range result.Warnings {
		b.WriteString("> **Warning:** " + warning + "\n\n")
	}

	if len(result.Findings) == 0 {
		if len(result.Warnings) > 0 {
			b.WriteString("No findings in the completed analysis.\n")
		} else {
			b.WriteString("No issues found.\n")
		}
		return b.String()
	}

	b.WriteString(fmt.Sprintf("### Findings (%d)\n\n", len(result.Findings)))
	for i, f := range result.Findings {
		loc := "General"
		if f.File != "" {
			if f.Line > 0 {
				loc = fmt.Sprintf("`%s:%d`", f.File, f.Line)
			} else {
				loc = fmt.Sprintf("`%s`", f.File)
			}
		}
		b.WriteString(fmt.Sprintf("%d. **[%s]** %s (Confidence: %.0f%%)\n", i+1, strings.ToUpper(string(f.Severity)), loc, f.Confidence*100))
		b.WriteString(fmt.Sprintf("   %s\n", f.Explanation))
		if f.SuggestedFix != "" {
			b.WriteString("   ```\n")
			b.WriteString(fmt.Sprintf("   %s\n", f.SuggestedFix))
			b.WriteString("   ```\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}
