/*
Copyright © 2026 hemanshulabs
*/
package cmd

import (
	"context"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/hemanshulabs/gh-forge/v4/internal/agent/provider"
	"github.com/hemanshulabs/gh-forge/v4/internal/agent/roles"
	"github.com/hemanshulabs/gh-forge/v4/internal/tui/constants"
)

var agentsCmd = &cobra.Command{
	Use:     "agents",
	Aliases: []string{"agent"},
	Short:   "Manage and inspect specialized AI agents",
	Long: `Specialized AI agents designed to assist developers and maintainers directly from the terminal.

Specialization comes from focused system prompts, scoped toolsets, and rigid output schemas.
Available agents:
  • Reviewer  (forge review): PR diff quality, code style, risk, missing tests, PR summary
  • Triage    (forge triage): Issue labeling, deduplication, priority, repro checks
  • Sentinel  (forge scan):   Security vulnerabilities, auth gaps, injection, race conditions
  • Diagnostics (forge diag): Root cause analysis from supplied CI logs and stack traces`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log.SetLevel(log.ErrorLevel)

		headerStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#02F9FB")).
			MarginBottom(1)

		sectionStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#42A0FA")).
			MarginTop(1)

		boxStyle := lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#6272A4")).
			Padding(0, 1)

		fmt.Println(headerStyle.Render(fmt.Sprintf("\n%s FORGE AI SPECIALIZED AGENTS", constants.AgentIcon)))

		fmt.Println(sectionStyle.Render("Lead Orchestrator:"))
		janitorCfg := roles.NewJanitorConfig()
		fmt.Printf("  %s %s %s\n    %s\n",
			lipgloss.NewStyle().Foreground(lipgloss.Color("#C77DFF")).Render(constants.JanitorIcon),
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Render("Janitor"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("#C77DFF")).Render("(Primary Orchestrator & Task Dispatcher)"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("#ABB2BF")).Render(janitorCfg.Description),
		)

		agentList := []struct {
			icon string
			name string
			cmd  string
			desc string
		}{
			{icon: constants.CodeReviewIcon, name: "Reviewer", cmd: "forge review [pr]", desc: roles.NewReviewerConfig().Description},
			{icon: constants.LabelsIcon, name: "Triage", cmd: "forge triage [issue]", desc: roles.NewTriageConfig().Description},
			{icon: constants.SecurityIcon, name: "Sentinel", cmd: "forge scan [pr]", desc: roles.NewSentinelConfig().Description},
			{icon: constants.WorkflowIcon, name: "Diagnostics", cmd: "forge diag --logs-file <path>", desc: roles.NewDiagnosticsConfig().Description},
		}

		fmt.Println(sectionStyle.Render("Specialized Execution Workers:"))
		var agentsText strings.Builder
		for _, a := range agentList {
			agentsText.WriteString(fmt.Sprintf("  %s %s %s\n    %s\n",
				lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B")).Render(a.icon),
				lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Render(a.name),
				lipgloss.NewStyle().Faint(true).Render("("+a.cmd+")"),
				lipgloss.NewStyle().Foreground(lipgloss.Color("#ABB2BF")).Render(a.desc),
			))
		}
		fmt.Print(agentsText.String())

		fmt.Println(sectionStyle.Render("Detected Backend Providers:"))
		reg := provider.NewRegistry()
		providers := reg.ListProviders(context.Background())

		for _, p := range providers {
			statusIcon := lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B")).Render(fmt.Sprintf("%s Available", constants.SuccessIcon))
			if !p.Available {
				statusIcon = lipgloss.NewStyle().Foreground(lipgloss.Color("#E06C75")).Render(fmt.Sprintf("%s Unavailable (%s)", constants.FailureIcon, p.Reason))
			}
			fmt.Printf("  • %-30s %s\n", lipgloss.NewStyle().Bold(true).Render(p.Name), statusIcon)
		}

		fmt.Println()
		helpNotice := boxStyle.Render("Tip: Run 'forge review --help', 'forge triage --help', or 'forge scan --help' to use an agent.")
		fmt.Println(helpNotice)
		fmt.Println()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(agentsCmd)
}
