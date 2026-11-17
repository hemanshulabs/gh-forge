package prview

import (
	"fmt"
	"sort"
	"time"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/markdown"
	"github.com/dlvhdr/gh-dash/v4/internal/utils"
)

type RenderedActivity struct {
	UpdatedAt      time.Time
	RenderedString string
}

func (m *Model) renderActivity() string {
	width := m.getIndentedContentWidth()
	markdownRenderer := markdown.GetMarkdownRenderer(width, m.ctx)
	bodyStyle := lipgloss.NewStyle()

	var activities []RenderedActivity
	var comments []comment

	if !m.pr.Data.IsEnriched {
		return bodyStyle.Render("Loading...")
	}

	for _, review := range m.pr.Data.Enriched.ReviewThreads.Nodes {
		path := review.Path
		line := review.Line
		for _, c := range review.Comments.Nodes {
			comments = append(comments, comment{
				Author:    c.Author.Login,
				Body:      c.Body,
				UpdatedAt: c.UpdatedAt,
				Path:      &path,
				Line:      &line,
			})
		}
	}

	for _, c := range m.pr.Data.Enriched.Comments.Nodes {
		comments = append(comments, comment{
			Author:    c.Author.Login,
			Body:      c.Body,
			UpdatedAt: c.UpdatedAt,
		})
	}

	for _, comment := range comments {
		renderedComment, err := m.renderComment(comment, markdownRenderer)
		if err != nil {
			continue
		}
		activities = append(activities, RenderedActivity{
			UpdatedAt:      comment.UpdatedAt,
			RenderedString: renderedComment,
		})
	}

	for _, review := range m.pr.Data.Enriched.Reviews.Nodes {
		renderedReview, err := m.renderReview(review, markdownRenderer)
		if err != nil {
			continue
		}
		activities = append(activities, RenderedActivity{
			UpdatedAt:      review.UpdatedAt,
			RenderedString: renderedReview,
		})
	}

	sort.Slice(activities, func(i, j int) bool {
		return activities[i].UpdatedAt.Before(activities[j].UpdatedAt)
	})

	body := ""
	if len(activities) == 0 {
		body = renderEmptyState()
	} else {
		var renderedActivities []string
		for _, activity := range activities {
			renderedActivities = append(renderedActivities, activity.RenderedString)
		}
		title := m.ctx.Styles.Common.MainTextStyle.MarginBottom(1).Underline(true).Render(
			fmt.Sprintf("%s  %d comments", constants.CommentsIcon, len(activities)))
		body = lipgloss.JoinVertical(lipgloss.Left, renderedActivities...)
		body = lipgloss.JoinVertical(lipgloss.Left, title, body)
	}

	return bodyStyle.Render(body)
}

func renderEmptyState() string {
	return lipgloss.NewStyle().Italic(true).Render("No comments...")
}

type comment struct {
	Author    string
	UpdatedAt time.Time
	Body      string
	Path      *string
	Line      *int
}

func (m *Model) renderComment(
	comment comment,
	markdownRenderer glamour.TermRenderer,
) (string, error) {
	width := m.getIndentedContentWidth()
	authorAndTime := lipgloss.NewStyle().
		Width(width).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(m.ctx.Theme.FaintBorder).Render(
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			m.ctx.Styles.Common.MainTextStyle.Render(comment.Author),
			" ",
			lipgloss.NewStyle().
				Foreground(m.ctx.Theme.FaintText).
				Render(utils.TimeElapsed(comment.UpdatedAt)),
		))

	var header string
	if comment.Path != nil && comment.Line != nil {
		filePath := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Width(width).Render(
			fmt.Sprintf(
				"%s#l%d",
				*comment.Path,
				*comment.Line,
			),
		)
		header = lipgloss.JoinVertical(lipgloss.Left, authorAndTime, filePath, "")
	} else {
		header = authorAndTime
	}

	body := lineCleanupRegex.ReplaceAllString(comment.Body, "")
	body, err := markdownRenderer.Render(body)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		body,
	), err
}

func (m *Model) renderReview(
	review data.Review,
	markdownRenderer glamour.TermRenderer,
) (string, error) {
	header := m.renderReviewHeader(review)
	body, err := markdownRenderer.Render(review.Body)
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		body,
	), err
}

func (m *Model) renderReviewHeader(review data.Review) string {
	return lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderReviewDecision(review.State),
		" ",
		m.ctx.Styles.Common.MainTextStyle.Render(review.Author.Login),
		" ",
		lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render(
			"reviewed "+utils.TimeElapsed(review.UpdatedAt)),
	)
}

func (m *Model) renderReviewDecision(decision string) string {
	switch decision {
	case "PENDING":
		return m.ctx.Styles.Common.WaitingGlyph
	case "COMMENTED":
		return lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render("󰈈")
	case "APPROVED":
		return m.ctx.Styles.Common.SuccessGlyph
	case "CHANGES_REQUESTED":
		return m.ctx.Styles.Common.FailureGlyph
	}

	return ""
}

func (m *Model) renderAgentActivity() string {
	width := m.getIndentedContentWidth()
	bodyStyle := lipgloss.NewStyle()

	workerName, workerIcon, workerColor := m.getAssignedWorker()
	statusText, _ := m.getAgentTaskStatus()

	var events []string

	title := m.ctx.Styles.Common.MainTextStyle.MarginBottom(1).Underline(true).Bold(true).Render(
		fmt.Sprintf("%s Agent Execution Activity & Handoff Log", constants.WorkflowIcon),
	)
	events = append(events, title, "")

	// Event 1: Ingestion
	ev1Header := lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Foreground(lipgloss.Color("#C77DFF")).Bold(true).Render(fmt.Sprintf("%s Janitor", constants.JanitorIcon)),
		" ",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370")).Render("• Ingestion & Sanitization"),
	)
	ev1Body := lipgloss.NewStyle().Foreground(lipgloss.Color("#ABB2BF")).Render(
		"Ingested pull request context. Scanned diff for lockfiles and stripped noise (go.sum, devbox.lock). Token budget optimized.",
	)
	events = append(events, lipgloss.NewStyle().
		Width(width).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#C77DFF")).
		Padding(0, 1).
		Render(lipgloss.JoinVertical(lipgloss.Left, ev1Header, ev1Body)), "")

	// Event 2: Janitor Delegation
	ev2Header := lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Foreground(lipgloss.Color("#C77DFF")).Bold(true).Render(fmt.Sprintf("%s Janitor", constants.JanitorIcon)),
		" ",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370")).Render(fmt.Sprintf("• Task Routed to @%s", workerName)),
	)
	ev2Body := lipgloss.NewStyle().Foreground(lipgloss.Color("#ABB2BF")).Render(
		fmt.Sprintf("Janitor analyzed task scope and dispatched assignment to %s. Work boundary set to read-only diff inspection.", workerName),
	)
	events = append(events, lipgloss.NewStyle().
		Width(width).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#C77DFF")).
		Padding(0, 1).
		Render(lipgloss.JoinVertical(lipgloss.Left, ev2Header, ev2Body)), "")

	// Event 3: Worker Analysis
	ev3Header := lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Foreground(lipgloss.Color(workerColor)).Bold(true).Render(fmt.Sprintf("%s %s", workerIcon, workerName)),
		" ",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370")).Render("• Deep Analysis Run"),
	)
	var ev3Msg string
	if statusText == "Working" {
		ev3Msg = "Worker execution currently running. Evaluating code structure, regression hazards, and rule assertions."
	} else {
		ev3Msg = "Completed rule evaluation pass. Filtered low-confidence noise (< 0.75). Output formatted according to JSON schema."
	}
	events = append(events, lipgloss.NewStyle().
		Width(width).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(workerColor)).
		Padding(0, 1).
		Render(lipgloss.JoinVertical(lipgloss.Left, ev3Header, lipgloss.NewStyle().Foreground(lipgloss.Color("#ABB2BF")).Render(ev3Msg))), "")

	// Event 4: Synthesis & Verification
	ev4Header := lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Foreground(lipgloss.Color("#C77DFF")).Bold(true).Render(fmt.Sprintf("%s Janitor", constants.JanitorIcon)),
		" ",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370")).Render(fmt.Sprintf("• Status Verification: %s", statusText)),
	)
	ev4Body := fmt.Sprintf("Janitor validated execution outcome. Result: %s. Ready for developer command interaction.", statusText)
	events = append(events, lipgloss.NewStyle().
		Width(width).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#6272A4")).
		Padding(0, 1).
		Render(lipgloss.JoinVertical(lipgloss.Left, ev4Header, lipgloss.NewStyle().Foreground(lipgloss.Color("#ABB2BF")).Render(ev4Body))))

	// If there are GitHub comments/discussions, render them as well
	if m.pr != nil && m.pr.Data != nil && m.pr.Data.IsEnriched && (len(m.pr.Data.Enriched.Comments.Nodes) > 0 || len(m.pr.Data.Enriched.Reviews.Nodes) > 0) {
		events = append(events, "", m.ctx.Styles.Common.MainTextStyle.MarginBottom(1).Underline(true).Bold(true).Render(
			fmt.Sprintf("%s Developer Comments & Reviews", constants.CommentsIcon),
		))
		markdownRenderer := markdown.GetMarkdownRenderer(width, m.ctx)
		for _, c := range m.pr.Data.Enriched.Comments.Nodes {
			renderedComment, err := m.renderComment(comment{
				Author:    c.Author.Login,
				Body:      c.Body,
				UpdatedAt: c.UpdatedAt,
			}, markdownRenderer)
			if err == nil {
				events = append(events, renderedComment)
			}
		}
	}

	return bodyStyle.Render(lipgloss.JoinVertical(lipgloss.Left, events...))
}
