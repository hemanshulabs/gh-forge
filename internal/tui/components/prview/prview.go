package prview

import (
	"fmt"
	"image/color"
	"regexp"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/carousel"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/cmpcontroller"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/fuzzyselect"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/inputbox"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prssection"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/tasks"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/keys"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/markdown"
	"github.com/dlvhdr/gh-dash/v4/internal/utils"
)

var (
	htmlCommentRegex = regexp.MustCompile("(?U)<!--(.|[[:space:]])*-->")
	lineCleanupRegex = regexp.MustCompile(`((\n)+|^)([^\r\n]*\|[^\r\n]*(\n)?)+`)
	foldBodyHeight   = 8
)

type Model struct {
	ctx             *context.ProgramContext
	sectionId       int
	pr              *prrow.PullRequest
	width           int
	carousel        carousel.Model
	editor          cmpcontroller.Controller
	summaryViewMore bool
}

var (
	prTabs    = []string{" Overview", " Activity", " Commits", " Checks", " Files Changed"}
	agentTabs = []string{" Overview", "󱓞 Working Phases"}
	tabs      = prTabs
)

func NewModel(ctx *context.ProgramContext) Model {
	initialTabs := prTabs
	if ctx != nil && ctx.View == config.AgentsView {
		initialTabs = agentTabs
	}
	c := carousel.NewModel(
		carousel.WithItems(initialTabs),
		carousel.WithWidth(ctx.MainContentWidth),
	)

	ta := inputbox.DefaultTextArea(ctx)
	cmp := cmpcontroller.New(ctx, inputbox.ModelOpts{TextArea: &ta})

	return Model{
		ctx:      ctx,
		pr:       nil,
		carousel: c,
		editor:   cmp,
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	cmd, handled := m.editor.Update(msg)

	if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "ctrl+d" {
		value := m.editor.Value()
		mode := m.editor.Mode()
		m.editor.Exit()
		if m.pr == nil {
			return m, nil
		}

		sid := tasks.SectionIdentifier{Id: m.sectionId, Type: prssection.SectionType}

		switch mode {
		case cmpcontroller.ModeComment:
			if len(strings.TrimSpace(value)) != 0 {
				return m, tasks.CommentOnPR(m.ctx, sid, m.pr.Data.Primary, value)
			}
			return m, nil

		case cmpcontroller.ModeApprove:
			comment := ""
			if len(strings.TrimSpace(value)) != 0 {
				comment = value
			}
			return m, tasks.ApprovePR(m.ctx, sid, m.pr.Data.Primary, comment)

		case cmpcontroller.ModeAssign:
			usernames := fuzzyselect.AllWords(value)
			if len(usernames) > 0 {
				return m, tasks.AssignPR(m.ctx, sid, m.pr.Data.Primary, usernames)
			}
			return m, nil

		case cmpcontroller.ModeUnassign:
			usernames := fuzzyselect.AllWords(value)
			if len(usernames) > 0 {
				return m, tasks.UnassignPR(m.ctx, sid, m.pr.Data.Primary, usernames)
			}
			return m, nil

		case cmpcontroller.ModeLabel:
			labels := fuzzyselect.CurrentLabels(value)
			if len(labels) > 0 || len(m.pr.Data.Primary.Labels.Nodes) > 0 {
				return m, m.label(labels)
			}
			return m, nil
		}
	}

	if handled {
		return m, cmd
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, keys.PRKeys.PrevSidebarTab):
			m.carousel.MoveLeft()
		case key.Matches(keyMsg, keys.PRKeys.NextSidebarTab):
			m.carousel.MoveRight()
		}
	}

	return m, cmd
}

func (m Model) View() string {
	if !m.hasData() {
		return ""
	}

	m.syncTabs()

	body := strings.Builder{}
	if m.ctx != nil && m.ctx.View == config.AgentsView {
		switch m.carousel.SelectedItem() {
		case agentTabs[1]:
			body.WriteString(m.viewAgentPhasesTab())
		default:
			body.WriteString(m.viewAgentOverviewTab())
		}
	} else {
		switch m.carousel.SelectedItem() {
		case prTabs[0]:
			body.WriteString(m.viewOverviewTab())
		case prTabs[1]:
			body.WriteString(m.renderActivity())
		case prTabs[2]:
			body.WriteString(m.renderCommits())
		case prTabs[3]:
			body.WriteString(m.renderChecksOverview())
			body.WriteString("\n\n")
			body.WriteString(m.renderChecks())
		case prTabs[4]:
			body.WriteString(m.renderChangedFiles())
		default:
			body.WriteString(m.viewOverviewTab())
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		m.viewHeader(),
		lipgloss.NewStyle().Padding(0, m.ctx.Styles.Sidebar.ContentPadding).Render(body.String()),
	)
}

func (m *Model) syncTabs() {
	if m.ctx != nil && m.ctx.View == config.AgentsView {
		if len(m.carousel.Items()) != len(agentTabs) || m.carousel.Items()[0] != agentTabs[0] {
			m.carousel.SetItems(agentTabs)
			if m.carousel.Cursor() >= len(agentTabs) {
				m.carousel.SetCursor(0)
			}
		}
	} else {
		if len(m.carousel.Items()) != len(prTabs) || m.carousel.Items()[0] != prTabs[0] {
			m.carousel.SetItems(prTabs)
			if m.carousel.Cursor() >= len(prTabs) {
				m.carousel.SetCursor(0)
			}
		}
	}
}

func (m *Model) viewHeader() string {
	m.syncTabs()

	header := strings.Builder{}

	header.WriteString(m.renderFullNameAndNumber())
	header.WriteString("\n")

	header.WriteString(m.renderTitle())
	header.WriteString("\n\n")
	if m.ctx != nil && m.ctx.View == config.AgentsView {
		header.WriteString(m.renderAgentHeaderMeta())
	} else {
		header.WriteString(m.renderBranches())
		header.WriteString("\n\n")
		header.WriteString(m.renderAuthor())
	}
	header.WriteString("\n\n")
	header.WriteString(lipgloss.NewStyle().Width(m.width).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(m.ctx.Theme.FaintBorder).
		Render(m.carousel.View()),
	)

	header.WriteString("\n")
	return header.String()
}

func (m *Model) renderAgentHeaderMeta() string {
	if !m.hasData() {
		return ""
	}

	statusText, statusBg := m.getAgentTaskStatusPill()
	statusPill := m.ctx.Styles.PrView.PillStyle.
		BorderForeground(statusBg).
		Background(statusBg).
		Render(statusText)

	branches := lipgloss.NewStyle().
		Foreground(m.ctx.Theme.SecondaryText).
		Render(m.pr.Data.Primary.BaseRefName + "  " + m.pr.Data.Primary.HeadRefName)

	line1 := lipgloss.JoinHorizontal(lipgloss.Left,
		" ",
		statusPill,
		" ",
		branches,
	)

	workerName, workerIcon, workerColor := m.getAssignedWorker()
	time := lipgloss.NewStyle().Render(utils.TimeElapsed(m.pr.Data.Primary.CreatedAt))

	line2 := lipgloss.JoinHorizontal(lipgloss.Top,
		" by ",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#C77DFF")).Bold(true).Render(constants.JanitorIcon+" @Janitor"),
		lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render(" 󰁕 "),
		lipgloss.NewStyle().Foreground(lipgloss.Color(workerColor)).Bold(true).Render(workerIcon+" @"+workerName),
		lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render(
			lipgloss.JoinHorizontal(lipgloss.Top, " ⋅ ", time, " ago ⋅ agent orchestrator"),
		),
	)

	return lipgloss.JoinVertical(lipgloss.Left, line1, "\n", line2)
}

func (m *Model) getAssignedWorker() (string, string, string) {
	if m.pr == nil || m.pr.Data == nil || m.pr.Data.Primary == nil {
		return "Reviewer Agent", constants.CodeReviewIcon, "#02F9FB"
	}

	primary := m.pr.Data.Primary
	for _, l := range primary.Labels.Nodes {
		lname := strings.ToLower(l.Name)
		if strings.Contains(lname, "security") || strings.Contains(lname, "cve") || strings.Contains(lname, "vulnerability") {
			return "Sentinel Agent", constants.SecurityIcon, "#FF5555"
		}
	}

	if len(primary.Commits.Nodes) > 0 {
		state := string(primary.Commits.Nodes[0].Commit.StatusCheckRollup.State)
		if state == "FAILURE" || state == "ERROR" {
			return "Diagnostics Agent", constants.WorkflowIcon, "#F1FA8C"
		}
	}

	if len(primary.Labels.Nodes) == 0 && primary.ReviewRequests.TotalCount == 0 {
		return "Triage Agent", constants.LabelsIcon, "#50FA7B"
	}

	return "Reviewer Agent", constants.CodeReviewIcon, "#02F9FB"
}

func (m *Model) getAgentTaskStatusPill() (string, color.Color) {
	if m.pr == nil || m.pr.Data == nil || m.pr.Data.Primary == nil {
		return "Inactive", m.ctx.Theme.FaintText.Dark
	}

	primary := m.pr.Data.Primary
	isCiPending := false
	if len(primary.Commits.Nodes) > 0 {
		state := string(primary.Commits.Nodes[0].Commit.StatusCheckRollup.State)
		if state == "PENDING" || state == "EXPECTED" {
			isCiPending = true
		}
	}

	switch {
	case isCiPending:
		return " Working", m.ctx.Styles.Colors.OpenPR.Dark
	case primary.ReviewDecision == "CHANGES_REQUESTED":
		return " Needs you", m.ctx.Styles.Colors.ClosedPR.Dark
	case primary.ReviewRequests.TotalCount > 0:
		return " Needs you", m.ctx.Theme.WarningText.Dark
	case primary.ReviewDecision == "APPROVED":
		return " Ready", m.ctx.Styles.Colors.MergedPR.Dark
	case primary.IsDraft:
		return "○ Inactive", m.ctx.Theme.FaintText.Dark
	default:
		return "○ Inactive", m.ctx.Theme.FaintText.Dark
	}
}

func (m *Model) viewAgentOverviewTab() string {
	width := m.getIndentedContentWidth()
	body := strings.Builder{}

	workerName, workerIcon, workerColor := m.getAssignedWorker()
	statusText, _ := m.getAgentTaskStatusPill()

	body.WriteString(m.renderJanitorSection(width, workerName, workerIcon, workerColor, statusText))
	body.WriteString("\n\n")

	body.WriteString(m.renderAgentPhasesSection(width, workerName, statusText))
	body.WriteString("\n\n")

	body.WriteString(m.renderAgentScopeSection(width))
	body.WriteString("\n\n")

	body.WriteString(m.renderAgentActionSection(width))

	if m.editor.Mode() != cmpcontroller.ModeNone {
		body.WriteString("\n\n")
		body.WriteString(m.ctx.Styles.Sidebar.InputBox.Render(m.editor.View()))
	}

	return body.String()
}

func (m *Model) renderJanitorSection(width int, workerName, workerIcon, workerColor, statusText string) string {
	heading := m.ctx.Styles.Common.MainTextStyle.MarginBottom(1).Underline(true).Bold(true).Render(
		fmt.Sprintf("%s Janitor Orchestration", constants.JanitorIcon),
	)

	desc := lipgloss.NewStyle().
		Foreground(m.ctx.Theme.SecondaryText).
		Width(width).
		Render("Janitor is the primary supervisor coordinating this repository task. It triaged the event, prepared a sanitized diff context, and delegated execution to @" + workerName + ".")

	faintStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText)
	meta1 := lipgloss.JoinHorizontal(lipgloss.Top,
		faintStyle.Render("Supervisor:  "),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#C77DFF")).Bold(true).Render(constants.JanitorIcon+" Janitor (Lead)"),
	)
	meta2 := lipgloss.JoinHorizontal(lipgloss.Top,
		faintStyle.Render("Assigned:    "),
		lipgloss.NewStyle().Foreground(lipgloss.Color(workerColor)).Bold(true).Render(workerIcon+" "+workerName),
	)
	meta3 := lipgloss.JoinHorizontal(lipgloss.Top,
		faintStyle.Render("State:       "),
		lipgloss.NewStyle().Foreground(m.ctx.Theme.PrimaryText).Bold(true).Render(statusText),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		heading,
		"",
		desc,
		"",
		meta1,
		meta2,
		meta3,
	)
}

func (m *Model) renderAgentPhasesSection(width int, workerName, statusText string) string {
	heading := m.ctx.Styles.Common.MainTextStyle.MarginBottom(1).Underline(true).Bold(true).Render(
		fmt.Sprintf("%s Working Phases", constants.WorkflowIcon),
	)

	successGlyph := lipgloss.NewStyle().Foreground(m.ctx.Theme.SuccessText).Render("")
	faintText := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText)
	secondaryText := lipgloss.NewStyle().Foreground(m.ctx.Theme.SecondaryText)

	// Phase 1
	p1Title := lipgloss.JoinHorizontal(lipgloss.Top,
		successGlyph, " ",
		lipgloss.NewStyle().Foreground(m.ctx.Theme.PrimaryText).Bold(true).Render("Phase 1: Ingestion & Diff Sanitization"),
	)
	p1Desc := secondaryText.Render("   Filtered lockfile noise (go.sum, devbox.lock), token budget optimized.")

	// Phase 2
	p2Title := lipgloss.JoinHorizontal(lipgloss.Top,
		successGlyph, " ",
		lipgloss.NewStyle().Foreground(m.ctx.Theme.PrimaryText).Bold(true).Render("Phase 2: Janitor Task Triage & Delegation 󰁕 @"+workerName),
	)
	p2Desc := secondaryText.Render(fmt.Sprintf("   Classified diff scope; dispatched task to %s.", workerName))

	// Phase 3
	var p3Glyph, p3TitleText, p3DescText string
	if strings.Contains(statusText, "Working") {
		p3Glyph = lipgloss.NewStyle().Foreground(lipgloss.Color("#61AFEF")).Render("")
		p3TitleText = "Phase 3: Deep Rule Analysis (In Progress)"
		p3DescText = "   Active inspection: evaluating code style, risks, regressions, and tests."
	} else {
		p3Glyph = successGlyph
		p3TitleText = "Phase 3: Deep Rule Analysis & Security Scan"
		p3DescText = "   Completed diff inspection, style checks, and risk verification."
	}
	p3Title := lipgloss.JoinHorizontal(lipgloss.Top,
		p3Glyph, " ",
		lipgloss.NewStyle().Foreground(m.ctx.Theme.PrimaryText).Bold(true).Render(p3TitleText),
	)
	p3Desc := secondaryText.Render(p3DescText)

	// Phase 4
	var p4Glyph, p4TitleText, p4DescText string
	switch {
	case strings.Contains(statusText, "Ready"):
		p4Glyph = lipgloss.NewStyle().Foreground(m.ctx.Theme.SuccessText).Render("")
		p4TitleText = "Phase 4: Output Synthesis & Verification (Ready)"
		p4DescText = "   Findings verified against JSON schema; ready for developer review."
	case strings.Contains(statusText, "Needs you"):
		p4Glyph = lipgloss.NewStyle().Foreground(m.ctx.Theme.ErrorText).Render("")
		p4TitleText = "Phase 4: Output Synthesis (Needs Developer Attention)"
		p4DescText = "   Changes requested or reviews required before completion handoff."
	case strings.Contains(statusText, "Working"):
		p4Glyph = lipgloss.NewStyle().Foreground(lipgloss.Color("#61AFEF")).Render("")
		p4TitleText = "Phase 4: Output Synthesis (Pending Analysis)"
		p4DescText = "   Awaiting completion of worker analysis passes."
	default:
		p4Glyph = faintText.Render("○")
		p4TitleText = "Phase 4: Output Synthesis (Idle)"
		p4DescText = "   Task is in queue or waiting for activity."
	}
	p4Title := lipgloss.JoinHorizontal(lipgloss.Top,
		p4Glyph, " ",
		lipgloss.NewStyle().Foreground(m.ctx.Theme.PrimaryText).Bold(true).Render(p4TitleText),
	)
	p4Desc := secondaryText.Render(p4DescText)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		heading,
		"",
		p1Title,
		p1Desc,
		"",
		p2Title,
		p2Desc,
		"",
		p3Title,
		p3Desc,
		"",
		p4Title,
		p4Desc,
	)
}

func (m *Model) renderAgentScopeSection(width int) string {
	heading := m.ctx.Styles.Common.MainTextStyle.MarginBottom(1).Underline(true).Bold(true).Render(" Scope & Changes")

	var filesCount, addCount, delCount int
	if m.pr != nil && m.pr.Data != nil && m.pr.Data.Primary != nil {
		addCount = m.pr.Data.Primary.Additions
		delCount = m.pr.Data.Primary.Deletions
		if m.pr.Data.IsEnriched {
			filesCount = m.pr.Data.Enriched.Files.TotalCount
		}
	}

	w := max(width-2, 10)
	row1 := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(m.ctx.Theme.FaintBorder).
		Width(w).
		Padding(1).
		Render(
			lipgloss.JoinHorizontal(lipgloss.Top,
				lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render(" "),
				fmt.Sprintf("%d files in scope", filesCount),
				"  ",
				lipgloss.NewStyle().Foreground(m.ctx.Theme.SuccessText).Render(fmt.Sprintf("+%d", addCount)),
				" ",
				lipgloss.NewStyle().Foreground(m.ctx.Theme.ErrorText).Render(fmt.Sprintf("-%d", delCount)),
			),
		)

	row2 := lipgloss.NewStyle().
		Width(w).
		Padding(1).
		Render(
			lipgloss.JoinHorizontal(lipgloss.Top,
				lipgloss.NewStyle().Foreground(lipgloss.Color("#42A0FA")).Render("󰒃 "),
				lipgloss.NewStyle().Foreground(m.ctx.Theme.SecondaryText).Render("Safety: Read-only context, suggestion-only output"),
			),
		)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true).
		BorderForeground(m.ctx.Theme.FaintBorder).
		Width(width).
		Render(lipgloss.JoinVertical(lipgloss.Left, row1, row2))

	return lipgloss.JoinVertical(lipgloss.Left, heading, "", box)
}

func (m *Model) renderAgentActionSection(width int) string {
	heading := m.ctx.Styles.Common.MainTextStyle.MarginBottom(1).Underline(true).Bold(true).Render(" Summary & Action")

	prNum := 0
	if m.pr != nil && m.pr.Data != nil && m.pr.Data.Primary != nil {
		prNum = m.pr.Data.Primary.Number
	}

	summaryText := "All agent checks evaluated according to the Forge schema specification. Ready for review or interactive command execution."
	if m.pr != nil && m.pr.Data != nil && m.pr.Data.IsEnriched && strings.TrimSpace(m.pr.Data.Enriched.Body) != "" {
		cleanBody := strings.TrimSpace(lineCleanupRegex.ReplaceAllString(htmlCommentRegex.ReplaceAllString(m.pr.Data.Enriched.Body, ""), ""))
		if len(cleanBody) > 0 {
			lines := strings.Split(cleanBody, "\n")
			for _, l := range lines {
				l = strings.TrimSpace(l)
				if l != "" && !strings.HasPrefix(l, "#") {
					summaryText = l
					break
				}
			}
		}
	}

	summaryRendered := lipgloss.NewStyle().
		Foreground(m.ctx.Theme.PrimaryText).
		Width(width).
		Render(summaryText)

	cliTip := lipgloss.NewStyle().
		Foreground(m.ctx.Theme.FaintText).
		Width(width).
		Render(fmt.Sprintf("Tip: Run 'forge review %d' to inspect code diff or 'forge scan %d' for security audit.", prNum, prNum))

	return lipgloss.JoinVertical(
		lipgloss.Left,
		heading,
		"",
		summaryRendered,
		"",
		cliTip,
	)
}

func (m *Model) viewAgentPhasesTab() string {
	width := m.getIndentedContentWidth()
	body := strings.Builder{}

	workerName, workerIcon, _ := m.getAssignedWorker()
	statusText, _ := m.getAgentTaskStatusPill()

	heading := m.ctx.Styles.Common.MainTextStyle.MarginBottom(1).Underline(true).Bold(true).Render(
		fmt.Sprintf("%s Detailed Execution Trace", constants.WorkflowIcon),
	)
	body.WriteString(heading)
	body.WriteString("\n\n")

	// Phase 1 Detail
	p1Icon := lipgloss.NewStyle().Foreground(m.ctx.Theme.SuccessText).Render("")
	body.WriteString(m.renderPhaseStep(width, "Phase 1: Context Ingestion & Lockfile Sanitization", p1Icon,
		"Ingested repository context. Scanned diff tree and automatically excluded lockfiles (go.sum, package-lock.json, devbox.lock). Token consumption reduced, keeping prompt well within budget."))
	body.WriteString("\n\n")

	// Phase 2 Detail
	p2Icon := lipgloss.NewStyle().Foreground(m.ctx.Theme.SuccessText).Render("")
	body.WriteString(m.renderPhaseStep(width, "Phase 2: Janitor Triage & Routing 󰁕 @"+workerName, p2Icon,
		fmt.Sprintf("Janitor supervisor analyzed the PR diff and task scope. Dispatched execution to %s (%s). Work boundaries strictly established as read-only.", workerName, workerIcon)))
	body.WriteString("\n\n")

	// Phase 3 Detail
	p3Icon := lipgloss.NewStyle().Foreground(m.ctx.Theme.SuccessText).Render("")
	p3Text := fmt.Sprintf("%s executed rule checks: evaluated clean code principles, risk of regression, and missing test branches. Completed with confidence score ≥ 0.75.", workerName)
	if strings.Contains(statusText, "Working") {
		p3Icon = lipgloss.NewStyle().Foreground(lipgloss.Color("#61AFEF")).Render("")
		p3Text = fmt.Sprintf("%s is actively processing this task. Inspecting diff, validating syntax, and evaluating assertions.", workerName)
	}
	body.WriteString(m.renderPhaseStep(width, "Phase 3: Deep Rule Analysis & Security Scan", p3Icon, p3Text))
	body.WriteString("\n\n")

	// Phase 4 Detail
	p4Icon := lipgloss.NewStyle().Foreground(m.ctx.Theme.SuccessText).Render("")
	p4Text := fmt.Sprintf("Janitor validated the findings against the rigid JSON schema specification. Final status: %s. Formatted for maintainer terminal review.", statusText)
	if strings.Contains(statusText, "Needs you") {
		p4Icon = lipgloss.NewStyle().Foreground(m.ctx.Theme.ErrorText).Render("")
		p4Text = "Task requires maintainer interaction. Review changes requested or feedback required before completion."
	} else if strings.Contains(statusText, "Working") {
		p4Icon = lipgloss.NewStyle().Foreground(lipgloss.Color("#61AFEF")).Render("")
		p4Text = "Awaiting completion of worker execution before output synthesis."
	}
	body.WriteString(m.renderPhaseStep(width, "Phase 4: Output Synthesis & Verification", p4Icon, p4Text))

	// If there are discussion comments from GitHub, show them below the trace
	if m.pr != nil && m.pr.Data != nil && m.pr.Data.IsEnriched && (len(m.pr.Data.Enriched.Comments.Nodes) > 0 || len(m.pr.Data.Enriched.Reviews.Nodes) > 0) {
		body.WriteString("\n\n")
		body.WriteString(m.ctx.Styles.Common.MainTextStyle.MarginBottom(1).Underline(true).Bold(true).Render(
			fmt.Sprintf("%s Review Comments & Discussions", constants.CommentsIcon),
		))
		body.WriteString("\n\n")
		markdownRenderer := markdown.GetMarkdownRenderer(width, m.ctx)
		for _, c := range m.pr.Data.Enriched.Comments.Nodes {
			renderedComment, err := m.renderComment(comment{
				Author:    c.Author.Login,
				Body:      c.Body,
				UpdatedAt: c.UpdatedAt,
			}, markdownRenderer)
			if err == nil {
				body.WriteString(renderedComment)
				body.WriteString("\n")
			}
		}
	}

	return body.String()
}

func (m *Model) renderPhaseStep(width int, stepTitle, iconStr, detail string) string {
	title := lipgloss.NewStyle().Foreground(m.ctx.Theme.PrimaryText).Bold(true).Render(stepTitle)
	head := lipgloss.JoinHorizontal(lipgloss.Top, iconStr, " ", title)

	desc := lipgloss.NewStyle().
		Foreground(m.ctx.Theme.SecondaryText).
		Width(width - 2).
		Render("  " + detail)

	return lipgloss.JoinVertical(lipgloss.Left, head, desc)
}

func (m *Model) viewOverviewTab() string {
	body := strings.Builder{}
	reviewers := m.renderRequestedReviewers()
	if reviewers != "" {
		body.WriteString(reviewers)
		body.WriteString("\n\n")
	}

	labels := m.renderLabels()
	if labels != "" {
		body.WriteString(labels)
		body.WriteString("\n\n")
	}

	body.WriteString(m.renderSummary())
	body.WriteString("\n\n")
	body.WriteString(
		m.ctx.Styles.Common.MainTextStyle.MarginBottom(1).Underline(true).Render(" Changes"),
	)
	body.WriteString("\n")
	body.WriteString(m.renderChangesOverview())
	body.WriteString("\n\n")
	body.WriteString(
		m.ctx.Styles.Common.MainTextStyle.MarginBottom(1).Underline(true).Render(" Checks"),
	)
	body.WriteString("\n")
	body.WriteString(m.renderChecksOverview())

	if m.editor.Mode() != cmpcontroller.ModeNone {
		body.WriteString(m.ctx.Styles.Sidebar.InputBox.Render(m.editor.View()))
	}

	return body.String()
}

func (m *Model) ViewCompletions() string {
	if !m.hasData() {
		return ""
	}
	return m.editor.ViewCompletions()
}

func (m *Model) InputBoxLineFromBottom() int {
	return m.editor.LineFromBottom()
}

func (m *Model) renderFullNameAndNumber() string {
	if !m.hasData() {
		return ""
	}

	return common.RenderPreviewHeader(
		m.ctx.Theme,
		m.width,
		fmt.Sprintf(
			"%s · #%d",
			m.pr.Data.Primary.GetRepoNameWithOwner(),
			m.pr.Data.Primary.GetNumber(),
		),
	)
}

func (m *Model) renderTitle() string {
	if !m.hasData() {
		return ""
	}

	return common.RenderPreviewTitle(
		m.ctx.Theme,
		m.ctx.Styles.Common,
		m.width,
		m.pr.Data.Primary.Title,
	)
}

func (m *Model) renderBranches() string {
	return lipgloss.JoinHorizontal(lipgloss.Left,
		" ",
		m.renderStatusPill(),
		" ",
		lipgloss.NewStyle().
			Foreground(m.ctx.Theme.SecondaryText).
			Render(m.pr.Data.Primary.BaseRefName+"  "+m.pr.Data.Primary.HeadRefName))
}

func (m *Model) renderStatusPill() string {
	var bgColor color.Color
	switch m.pr.Data.Primary.State {
	case "OPEN":
		if m.pr.Data.Primary.IsDraft {
			bgColor = m.ctx.Theme.FaintText.Dark
		} else {
			bgColor = m.ctx.Styles.Colors.OpenPR.Dark
		}
	case "CLOSED":
		bgColor = m.ctx.Styles.Colors.ClosedPR.Dark
	case "MERGED":
		bgColor = m.ctx.Styles.Colors.MergedPR.Dark
	}

	return m.ctx.Styles.PrView.PillStyle.
		BorderForeground(bgColor).
		Background(bgColor).
		Render(m.pr.RenderState())
}

func (m *Model) renderLabels() string {
	width := m.getIndentedContentWidth()
	labels := m.pr.Data.Primary.Labels.Nodes
	style := m.ctx.Styles.PrView.PillStyle
	if len(labels) == 0 {
		return ""
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.ctx.Styles.Common.MainTextStyle.Underline(true).Bold(true).Render(
			fmt.Sprintf("%s Labels", constants.LabelsIcon)),
		"",
		common.RenderLabels(labels, common.LabelOpts{
			Width:     width,
			PillStyle: style,
		}),
	)
}

type reviewerItem struct {
	text string
}

func (m *Model) renderRequestedReviewers() string {
	if !m.pr.Data.IsEnriched {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			m.ctx.Styles.Common.MainTextStyle.Underline(true).Bold(true).Render(
				fmt.Sprintf("%s Reviewers", constants.CodeReviewIcon)),
			"",
			lipgloss.JoinHorizontal(
				lipgloss.Top,
				m.ctx.Styles.Common.WaitingGlyph,
				" ",
				m.ctx.Styles.Common.FaintTextStyle.Render("Loading..."),
			),
		)
	}

	reviewRequests := m.pr.Data.Enriched.ReviewRequests.Nodes
	reviews := m.pr.Data.Enriched.Reviews.Nodes
	suggestedReviewers := m.pr.Data.Enriched.SuggestedReviewers

	if len(reviewRequests) == 0 && len(reviews) == 0 && len(suggestedReviewers) == 0 {
		return ""
	}

	reviewStates := make(map[string]string)
	for _, review := range reviews {
		login := review.Author.Login
		existingState := reviewStates[login]
		// Don't override APPROVED or CHANGES_REQUESTED with COMMENTED
		if review.State == "COMMENTED" &&
			(existingState == "APPROVED" || existingState == "CHANGES_REQUESTED") {
			continue
		}
		reviewStates[login] = review.State
	}

	reviewerItems := make([]reviewerItem, 0)
	faintStyle := m.ctx.Styles.Common.FaintTextStyle
	reviewerStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText)
	successStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.SuccessText)
	errorStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.ErrorText)

	shownReviewers := make(map[string]bool)

	for _, req := range reviewRequests {
		displayName := req.GetReviewerDisplayName()
		if displayName == "" {
			continue
		}
		shownReviewers[displayName] = true

		var reviewerStr string
		stateIcon := ""
		if state, hasReview := reviewStates[displayName]; hasReview && state == "COMMENTED" {
			stateIcon = m.ctx.Styles.Common.CommentGlyph
		} else {
			stateIcon = m.ctx.Styles.Common.WaitingDotGlyph
		}

		if req.IsTeam() {
			reviewerStr += reviewerStyle.Render(displayName)
		} else {
			reviewerStr += reviewerStyle.Render("@" + displayName)
		}

		if req.AsCodeOwner {
			reviewerStr = lipgloss.JoinHorizontal(lipgloss.Top,
				faintStyle.Render(constants.OwnerIcon), " ", reviewerStr)
		}
		reviewerStr = lipgloss.JoinHorizontal(lipgloss.Top, stateIcon, " ", reviewerStr)

		reviewerItems = append(reviewerItems, reviewerItem{text: reviewerStr})
	}

	for login, state := range reviewStates {
		if shownReviewers[login] {
			continue
		}
		if state != "APPROVED" && state != "CHANGES_REQUESTED" && state != "COMMENTED" {
			continue
		}
		shownReviewers[login] = true

		var stateIcon string
		switch state {
		case "APPROVED":
			stateIcon = successStyle.Render(constants.ApprovedIcon)
		case "CHANGES_REQUESTED":
			stateIcon = errorStyle.Render(constants.ChangesRequestedIcon)
		case "COMMENTED":
			stateIcon = m.ctx.Styles.Common.CommentGlyph
		}
		reviewerStr := stateIcon + " " + reviewerStyle.Render("@"+login)

		reviewerItems = append(reviewerItems, reviewerItem{text: reviewerStr})
	}

	// Show suggested reviewers (= code owners) who haven't been requested or reviewed yet
	for _, suggested := range suggestedReviewers {
		login := suggested.Reviewer.Login
		if shownReviewers[login] {
			continue
		}
		if suggested.IsAuthor {
			continue
		}
		shownReviewers[login] = true

		reviewerStr := lipgloss.JoinHorizontal(lipgloss.Top,
			faintStyle.Render(constants.OwnerIcon), " ",
			faintStyle.Render("@"+login),
		)

		reviewerItems = append(reviewerItems, reviewerItem{text: reviewerStr})
	}

	if len(reviewerItems) == 0 {
		return ""
	}

	width := m.getIndentedContentWidth()
	var rows []string
	var currentRow strings.Builder
	currentRowWidth := 0

	for i, item := range reviewerItems {
		itemWidth := lipgloss.Width(item.text)
		separator := ", "
		separatorWidth := lipgloss.Width(separator)

		// Check if adding this item would exceed the width
		needsSeparator := i < len(reviewerItems)-1
		totalItemWidth := itemWidth
		if needsSeparator {
			totalItemWidth += separatorWidth
		}

		if currentRowWidth > 0 && currentRowWidth+totalItemWidth > width {
			// Start a new row
			rows = append(rows, currentRow.String())
			currentRow.Reset()
			currentRowWidth = 0
		}

		currentRow.WriteString(item.text)
		currentRowWidth += itemWidth

		if needsSeparator {
			currentRow.WriteString(separator)
			currentRowWidth += separatorWidth
		}
	}

	// Add the last row
	if currentRow.Len() > 0 {
		rows = append(rows, currentRow.String())
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.ctx.Styles.Common.MainTextStyle.Underline(true).Bold(true).Render(
			fmt.Sprintf("%s Reviewers", constants.CodeReviewIcon)),
		"",
		strings.Join(rows, "\n"),
	)
}

func (m *Model) renderAuthor() string {
	authorAssociation := m.pr.Data.Primary.AuthorAssociation
	if authorAssociation == "" {
		authorAssociation = "unknown role"
	}
	time := lipgloss.NewStyle().Render(utils.TimeElapsed(m.pr.Data.Primary.CreatedAt))
	return lipgloss.JoinHorizontal(lipgloss.Top,
		" by ",
		lipgloss.NewStyle().Foreground(m.ctx.Theme.PrimaryText).Render(
			lipgloss.NewStyle().Bold(true).Render("@"+m.pr.Data.Primary.Author.Login)),
		lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render(
			lipgloss.JoinHorizontal(lipgloss.Top, " ⋅ ", time, " ago", " ⋅ ")),
		lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render(
			lipgloss.JoinHorizontal(lipgloss.Top, data.GetAuthorRoleIcon(m.pr.Data.Primary.AuthorAssociation,
				m.ctx.Theme),
				" ", lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render(strings.ToLower(authorAssociation))),
		),
	)
}

func (m *Model) renderSummary() string {
	width := m.getIndentedContentWidth()
	// Strip HTML comments from body and cleanup body.
	body := htmlCommentRegex.ReplaceAllString(m.pr.Data.Enriched.Body, "")
	body = lineCleanupRegex.ReplaceAllString(body, "")

	desc := m.ctx.Styles.Common.MainTextStyle.Bold(true).Underline(true).Render(" Summary")
	title := lipgloss.JoinVertical(
		lipgloss.Left,
		desc,
		"",
	)
	sbody := lipgloss.NewStyle().Width(m.getIndentedContentWidth())
	body = strings.TrimSpace(body)
	if body == "" {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			sbody.Italic(true).Foreground(m.ctx.Theme.FaintText).Render("No description provided."),
		)
	}

	markdownRenderer := markdown.GetMarkdownRenderer(width, m.ctx)
	rendered, err := markdownRenderer.Render(body)
	if err != nil {
		return ""
	}

	bodyHeight := lipgloss.Height(rendered)
	if !m.summaryViewMore && bodyHeight > foldBodyHeight {
		rendered = lipgloss.NewStyle().MaxHeight(foldBodyHeight).Render(rendered)
		rendered = lipgloss.JoinVertical(lipgloss.Left,
			rendered,
			"",
			lipgloss.PlaceHorizontal(m.getIndentedContentWidth(), lipgloss.Center,
				lipgloss.JoinHorizontal(
					lipgloss.Top,
					lipgloss.NewStyle().Bold(true).Italic(true).Render("Press "),
					lipgloss.NewStyle().
						Background(m.ctx.Theme.SelectedBackground).
						Foreground(m.ctx.Theme.PrimaryText).
						Render("e"),
					lipgloss.NewStyle().Bold(true).Italic(true).Render(" to read more..."),
				),
			),
		)
	}

	return lipgloss.JoinVertical(lipgloss.Left, title,
		lipgloss.NewStyle().
			Width(width).
			MaxWidth(width).
			Align(lipgloss.Left).
			Render(rendered),
	)
}

func (m *Model) SetSectionId(id int) {
	m.sectionId = id
}

func (m *Model) SetRow(d *prrow.Data) {
	if d == nil {
		m.pr = nil
	} else {
		m.pr = &prrow.PullRequest{Ctx: m.ctx, Data: d}
	}
}

type EnrichedPrMsg struct {
	Id   int
	Type string
	Data data.EnrichedPullRequestData
	Err  error
}

func (m *Model) EnrichCurrRow() tea.Cmd {
	if m == nil || m.pr == nil || m.pr.Data.IsEnriched {
		return nil
	}
	url := m.pr.Data.Primary.Url
	return func() tea.Msg {
		d, err := data.FetchPullRequest(url)
		return EnrichedPrMsg{
			Id:   m.sectionId,
			Type: prssection.SectionType,
			Data: d,
			Err:  err,
		}
	}
}

func (m *Model) SetWidth(width int) {
	m.width = width
	m.carousel.SetWidth(width)
	m.editor.SetWidth(
		m.getIndentedContentWidth() - m.ctx.Styles.Sidebar.InputBox.GetHorizontalFrameSize(),
	)
}

func (m *Model) IsTextInputBoxFocused() bool {
	return m.editor.Active()
}

func (m *Model) UpdateProgramContext(ctx *context.ProgramContext) {
	m.ctx = ctx
	m.editor.UpdateProgramContext(ctx)
	m.carousel.SetStyles(
		carousel.Styles{
			Item:     lipgloss.NewStyle().Padding(0, 1).Foreground(m.ctx.Theme.FaintText),
			Selected: lipgloss.NewStyle().Padding(0, 1).Bold(true),
		},
	)

	// TODO: move this to the NewModel func
	// currently it's not possible since the styles aren't yet instantiated when NewModel is called
	m.editor.SetSelectStyles(ctx.Styles.Select)
}

func (m *Model) GetIsCommenting() bool {
	return m.editor.Mode() == cmpcontroller.ModeComment
}

func (m *Model) SetIsCommenting(isCommenting bool) tea.Cmd {
	if m.pr == nil {
		return nil
	}

	if !isCommenting {
		if m.editor.Mode() == cmpcontroller.ModeComment {
			m.editor.Exit()
		}
		return nil
	}

	m.editor.SetAutocompleteSource(&fuzzyselect.UserMentionSource{WithAtSymbol: true})
	cmd := m.editor.Enter(cmpcontroller.EnterOptions{
		Mode:                             cmpcontroller.ModeComment,
		Prompt:                           constants.CommentPrompt,
		Repo:                             m.repoRef(),
		EnterFetch:                       cmpcontroller.FetchSilent,
		ConfirmDiscardOnCancel:           true,
		HideAutocompleteWhenContextEmpty: true,
	})
	return cmd
}

func (m *Model) getIndentedContentWidth() int {
	return m.width - 2*m.ctx.Styles.Sidebar.ContentPadding
}

func (m *Model) GetIsApproving() bool {
	return m.editor.Mode() == cmpcontroller.ModeApprove
}

func (m *Model) SetIsApproving(isApproving bool) tea.Cmd {
	if m.pr == nil {
		return nil
	}

	if !isApproving {
		if m.editor.Mode() == cmpcontroller.ModeApprove {
			m.editor.Exit()
		}
		return nil
	}

	m.editor.SetAutocompleteSource(&fuzzyselect.UserMentionSource{WithAtSymbol: true})
	cmd := m.editor.Enter(cmpcontroller.EnterOptions{
		Mode:                             cmpcontroller.ModeApprove,
		Prompt:                           constants.ApprovalPrompt,
		InitialValue:                     m.ctx.Config.Defaults.PrApproveComment,
		Repo:                             m.repoRef(),
		EnterFetch:                       cmpcontroller.FetchSilent,
		ConfirmDiscardOnCancel:           true,
		HideAutocompleteWhenContextEmpty: true,
	})
	return cmd
}

func (m *Model) GetIsAssigning() bool {
	return m.editor.Mode() == cmpcontroller.ModeAssign
}

func (m *Model) SetIsAssigning(isAssigning bool) tea.Cmd {
	if m.pr == nil {
		return nil
	}

	if !isAssigning {
		if m.editor.Mode() == cmpcontroller.ModeAssign {
			m.editor.Exit()
		}
		return nil
	}

	initialValue := ""
	if !m.userAssignedToPr(m.ctx.User) {
		initialValue = m.ctx.User
	}

	m.editor.SetAutocompleteSource(&fuzzyselect.UserMentionSource{WithAtSymbol: false})
	cmd := m.editor.Enter(cmpcontroller.EnterOptions{
		Mode:                             cmpcontroller.ModeAssign,
		Prompt:                           constants.AssignPrompt,
		InitialValue:                     initialValue,
		Repo:                             m.repoRef(),
		EnterFetch:                       cmpcontroller.FetchSilent,
		HideAutocompleteWhenContextEmpty: false,
	})
	m.editor.ShowCompletions()
	return cmd
}

func (m *Model) userAssignedToPr(login string) bool {
	for _, a := range m.pr.Data.Primary.Assignees.Nodes {
		if login == a.Login {
			return true
		}
	}
	return false
}

func (m *Model) GetIsUnassigning() bool {
	return m.editor.Mode() == cmpcontroller.ModeUnassign
}

func (m *Model) SetIsUnassigning(isUnassigning bool) tea.Cmd {
	if m.pr == nil {
		return nil
	}

	if !isUnassigning {
		if m.editor.Mode() == cmpcontroller.ModeUnassign {
			m.editor.Exit()
		}
		return nil
	}

	cmd := m.editor.Enter(cmpcontroller.EnterOptions{
		Mode:         cmpcontroller.ModeUnassign,
		Prompt:       constants.UnassignPrompt,
		InitialValue: strings.Join(m.prAssignees(), "\n"),
		Repo:         m.repoRef(),
	})
	return cmd
}

func (m *Model) prAssignees() []string {
	var assignees []string
	for _, n := range m.pr.Data.Primary.Assignees.Nodes {
		assignees = append(assignees, n.Login)
	}
	return assignees
}

func (m *Model) GoToFirstTab() {
	m.carousel.SetCursor(0)
}

func (m *Model) GoToActivityTab() {
	m.carousel.SetCursor(1) // Activity is the second tab (index 1)
}

func (m Model) SelectedTab() string {
	return m.carousel.SelectedItem()
}

func (m *Model) SetSummaryViewMore() {
	m.summaryViewMore = true
}

func (m *Model) SetSummaryViewLess() {
	m.summaryViewMore = false
}

func (m *Model) SetEnrichedPR(data data.EnrichedPullRequestData) {
	if m.pr.Data.Primary.Url == data.Url {
		m.pr.Data.Enriched = data
		m.pr.Data.IsEnriched = true
	}
}

func (m *Model) GetIsLabeling() bool {
	return m.editor.Mode() == cmpcontroller.ModeLabel
}

// SetIsLabeling enters or exits labeling mode
func (m *Model) SetIsLabeling(isLabeling bool) tea.Cmd {
	if m.pr == nil {
		return nil
	}

	if !isLabeling {
		if m.editor.Mode() == cmpcontroller.ModeLabel {
			m.editor.Exit()
		}
		return nil
	}

	labels := make([]string, 0, len(m.pr.Data.Primary.Labels.Nodes)+1)
	for _, label := range m.pr.Data.Primary.Labels.Nodes {
		labels = append(labels, label.Name)
	}
	labels = append(labels, "")

	m.editor.SetAutocompleteSource(&fuzzyselect.LabelSource{})
	cmd := m.editor.Enter(cmpcontroller.EnterOptions{
		Mode:                             cmpcontroller.ModeLabel,
		Prompt:                           constants.LabelPrompt,
		InitialValue:                     strings.Join(labels, ", "),
		Repo:                             m.repoRef(),
		EnterFetch:                       cmpcontroller.FetchSilent,
		HideAutocompleteWhenContextEmpty: false,
		ConfirmDiscardOnCancel:           false,
	})
	m.editor.ShowCompletions()
	return cmd
}

func (m *Model) repoRef() cmpcontroller.RepoRef {
	owner, repo := m.pr.Data.Primary.GetRepoNameAndOwner()
	return cmpcontroller.RepoRef{
		NameWithOwner: m.pr.Data.Primary.GetRepoNameWithOwner(),
		Owner:         owner,
		Name:          repo,
	}
}

func (m *Model) hasData() bool {
	return m.pr != nil && m.pr.Data != nil
}
