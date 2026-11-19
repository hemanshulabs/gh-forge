package prssection

import (
	stdcontext "context"
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
	"charm.land/log/v2"

	"github.com/hemanshulabs/gh-forge/v4/internal/agent/provider"
	"github.com/hemanshulabs/gh-forge/v4/internal/config"
	"github.com/hemanshulabs/gh-forge/v4/internal/data"
	"github.com/hemanshulabs/gh-forge/v4/internal/tui/components/prrow"
	"github.com/hemanshulabs/gh-forge/v4/internal/tui/components/search"
	"github.com/hemanshulabs/gh-forge/v4/internal/tui/components/section"
	"github.com/hemanshulabs/gh-forge/v4/internal/tui/components/table"
	"github.com/hemanshulabs/gh-forge/v4/internal/tui/components/tasks"
	"github.com/hemanshulabs/gh-forge/v4/internal/tui/constants"
	"github.com/hemanshulabs/gh-forge/v4/internal/tui/context"
	"github.com/hemanshulabs/gh-forge/v4/internal/tui/keys"
	"github.com/hemanshulabs/gh-forge/v4/internal/utils"
)

const (
	SectionType      = "pr"
	AgentSectionType = "agents"
)

type Model struct {
	section.BaseModel
	Prs         []prrow.Data
	cfg         config.PrsSectionConfig
	HistoryOnly bool
}

func NewModel(
	id int,
	ctx *context.ProgramContext,
	cfg config.PrsSectionConfig,
	lastUpdated time.Time,
	createdAt time.Time,
) Model {
	m := Model{}
	m.cfg = cfg
	m.Prs = []prrow.Data{}
	m.BaseModel = section.NewModel(
		ctx,
		section.NewSectionOptions{
			Id:          id,
			Config:      cfg.ToSectionConfig(),
			Type:        SectionType,
			Columns:     GetSectionColumns(cfg, ctx),
			Singular:    m.GetItemSingularForm(),
			Plural:      m.GetItemPluralForm(),
			LastUpdated: lastUpdated,
			CreatedAt:   createdAt,
		},
	)

	return m
}

func GetAgentSectionColumns() []table.Column {
	return []table.Column{
		{
			Title: "Tasks",
			Grow:  utils.BoolPtr(true),
		},
		{
			Title: "Status",
			Width: utils.IntPtr(20),
		},
		{
			Title: "Updated",
			Width: utils.IntPtr(10),
		},
	}
}

func GetAgentHistoryColumns() []table.Column {
	return []table.Column{
		{Title: "Task", Grow: utils.BoolPtr(true)},
		{Title: "Role", Width: utils.IntPtr(13)},
		{Title: "Status", Width: utils.IntPtr(16)},
		{Title: "Agent reply", Width: utils.IntPtr(38)},
	}
}

func NewAgentModel(
	id int,
	ctx *context.ProgramContext,
	cfg config.PrsSectionConfig,
	lastUpdated time.Time,
	createdAt time.Time,
) Model {
	m := NewModel(id, ctx, cfg, lastUpdated, createdAt)
	m.Type = AgentSectionType
	if strings.EqualFold(cfg.Title, "history") {
		m.HistoryOnly = true
		m.Table.Columns = GetAgentHistoryColumns()
	} else {
		m.Table.Columns = GetAgentSectionColumns()
	}
	m.Table.SetContentHeight(1)
	m.Table.DisableSelectedBackground = true
	m.SingularForm = "task"
	m.PluralForm = "tasks"
	m.SearchBar = search.NewModel(ctx, search.SearchOptions{
		Placeholder: "Filter tasks (tab/shift+tab to switch section)...",
	})
	return m
}

func (m Model) buildAgentRows() []table.Row {
	if m.HistoryOnly {
		var rows []table.Row
		for i, task := range m.agentHistoryTasks() {
			rows = append(rows, m.toAgentHistoryRow(task, m.Table.GetCurrItem() == i))
		}
		if rows == nil {
			return []table.Row{}
		}
		return rows
	}
	var rows []table.Row
	currItem := m.Table.GetCurrItem()
	for i, currPr := range m.Prs {
		rows = append(rows, m.toAgentRow(currPr, currItem == i))
	}
	for i, task := range m.agentQueueTasks() {
		rows = append(rows, m.toAgentQueueRow(task, currItem == len(m.Prs)+i))
	}
	if rows == nil {
		rows = []table.Row{}
	}
	return rows
}

func (m Model) agentHistoryTasks() []context.AgentTask {
	tasks := make([]context.AgentTask, 0, len(m.Ctx.AgentTasks))
	for _, task := range m.Ctx.AgentTasks {
		tasks = append(tasks, task)
	}
	slices.SortFunc(tasks, func(a, b context.AgentTask) int {
		if a.UpdatedAt.After(b.UpdatedAt) {
			return -1
		}
		if a.UpdatedAt.Before(b.UpdatedAt) {
			return 1
		}
		return 0
	})
	return tasks
}

func (m Model) toAgentHistoryRow(task context.AgentTask, selected bool) table.Row {
	style := lipgloss.NewStyle().Foreground(m.Ctx.Theme.SecondaryText)
	if selected {
		style = style.Foreground(m.Ctx.Theme.PrimaryText).Bold(true)
	}
	target := "PR"
	if task.Kind == "issue" {
		target = "Issue"
	}
	text := fmt.Sprintf("%s #%d  %s  ·  %s", target, task.Number, task.Repo, task.Title)
	reply := strings.TrimSpace(strings.ReplaceAll(task.Summary, "\n", " "))
	if reply == "" {
		reply = task.Error
	}
	if len(reply) > 120 {
		reply = reply[:117] + "..."
	}
	return table.Row{style.Render(text), task.Role, agentTaskStatusLabel(task.Status), reply}
}

func agentTaskStatusLabel(status string) string {
	switch status {
	case "needs_input":
		return "needs input"
	case "choose CLI":
		return "waiting for CLI"
	default:
		return status
	}
}

func (m Model) toAgentRow(pr prrow.Data, isSelected bool) table.Row {
	if pr.Primary == nil {
		return table.Row{"", "", ""}
	}

	var status string
	var statusFg compat.AdaptiveColor
	var icon string

	isCiPending := false
	if len(pr.Primary.Commits.Nodes) > 0 {
		state := string(pr.Primary.Commits.Nodes[0].Commit.StatusCheckRollup.State)
		if state == "PENDING" || state == "EXPECTED" {
			isCiPending = true
		}
	}

	switch {
	case isCiPending:
		status = "Checks pending"
		statusFg = m.Ctx.Styles.Colors.OpenPR
		icon = constants.WaitingIcon
	case pr.Primary.ReviewDecision == "CHANGES_REQUESTED":
		status = "Changes requested"
		statusFg = m.Ctx.Theme.ErrorText
		icon = constants.ActionRequiredIcon
	case pr.Primary.ReviewRequests.TotalCount > 0:
		status = "Review requested"
		statusFg = m.Ctx.Theme.WarningText
		icon = constants.ActionRequiredIcon
	case pr.Primary.ReviewDecision == "APPROVED":
		status = "Approved"
		statusFg = m.Ctx.Theme.SuccessText
		icon = constants.ApprovedIcon
	case pr.Primary.IsDraft:
		status = "Draft"
		statusFg = m.Ctx.Theme.FaintText
		icon = constants.DraftIcon
	default:
		status = "Open"
		statusFg = m.Ctx.Theme.FaintText
		icon = constants.DraftIcon
	}

	iconStyled := lipgloss.NewStyle().Foreground(statusFg).Render(icon)
	titleText := pr.Primary.Title
	titleStyle := lipgloss.NewStyle()
	if isSelected {
		titleStyle = titleStyle.Foreground(m.Ctx.Theme.PrimaryText).Bold(true)
	} else {
		titleStyle = titleStyle.Foreground(m.Ctx.Theme.SecondaryText)
	}
	taskCell := fmt.Sprintf("%s %s", iconStyled, titleStyle.Render(titleText))

	statusCell := lipgloss.NewStyle().Foreground(statusFg).Render(status)
	if task, ok := m.Ctx.AgentTasks[context.AgentTaskKey("pr", pr.Primary.Repository.NameWithOwner, pr.Primary.Number)]; ok {
		statusCell = lipgloss.NewStyle().Foreground(m.Ctx.Theme.PrimaryText).Render(task.Role + ":" + agentTaskStatusLabel(task.Status))
	}

	elapsed := utils.TimeElapsed(pr.Primary.UpdatedAt)
	var updatedText string
	if elapsed == "now" {
		updatedText = "just now"
	} else {
		updatedText = fmt.Sprintf("%s ago", elapsed)
	}
	updatedCell := lipgloss.NewStyle().Foreground(m.Ctx.Theme.FaintText).Render(updatedText)

	return table.Row{
		taskCell,
		statusCell,
		updatedCell,
	}
}

type AgentQueueRow struct{ Task context.AgentTask }

func (r *AgentQueueRow) GetRepoNameWithOwner() string { return r.Task.Repo }
func (r *AgentQueueRow) GetTitle() string             { return r.Task.Title }
func (r *AgentQueueRow) GetNumber() int               { return r.Task.Number }
func (r *AgentQueueRow) GetUrl() string {
	return fmt.Sprintf("https://github.com/%s/issues/%d", r.Task.Repo, r.Task.Number)
}
func (r *AgentQueueRow) GetUpdatedAt() time.Time { return r.Task.UpdatedAt }

func (m Model) agentQueueTasks() []context.AgentTask {
	var found []context.AgentTask
	for _, task := range m.Ctx.AgentTasks {
		if task.SectionType != AgentSectionType {
			continue
		}
		if task.Kind == "issue" {
			found = append(found, task)
			continue
		}
		present := false
		for _, pr := range m.Prs {
			if pr.Primary != nil && pr.Primary.Number == task.Number && pr.Primary.Repository.NameWithOwner == task.Repo {
				present = true
				break
			}
		}
		if !present {
			found = append(found, task)
		}
	}
	slices.SortFunc(found, func(a, b context.AgentTask) int {
		if a.UpdatedAt.Before(b.UpdatedAt) {
			return -1
		}
		if a.UpdatedAt.After(b.UpdatedAt) {
			return 1
		}
		return 0
	})
	return found
}

func (m Model) toAgentQueueRow(task context.AgentTask, selected bool) table.Row {
	titleStyle := lipgloss.NewStyle().Foreground(m.Ctx.Theme.SecondaryText)
	if selected {
		titleStyle = titleStyle.Foreground(m.Ctx.Theme.PrimaryText).Bold(true)
	}
	return table.Row{titleStyle.Render(fmt.Sprintf("%s #%d %s", task.Repo, task.Number, task.Title)), task.Role + ":" + agentTaskStatusLabel(task.Status), ""}
}

func (m *Model) Update(msg tea.Msg) (section.Section, tea.Cmd) {
	var cmd tea.Cmd
	var err error

	switch msg := msg.(type) {
	case tea.KeyMsg:

		if m.IsSearchFocused() {
			switch msg.String() {
			case "ctrl+c", "esc":
				m.SearchBar.SetValue(m.SearchValue)
				blinkCmd := m.SetIsSearching(false)
				return m, blinkCmd

			case "enter":
				m.SearchValue = m.SearchBar.Value()
				m.SyncSmartFilterWithSearchValue()
				m.SetIsSearching(false)
				m.ResetRows()
				return m, tea.Batch(m.FetchNextPageSectionRows()...)
			}

			break
		}

		if m.IsPromptConfirmationFocused() {
			switch msg.String() {
			case "ctrl+c", "esc":
				m.PromptConfirmationBox.Reset()
				cmd = m.SetIsPromptConfirmationShown(false)
				return m, cmd

			case "enter":
				input := m.PromptConfirmationBox.Value()
				action := m.GetPromptConfirmationAction()
				pr := m.GetCurrRow()
				if action == "assignAgent" || action == "agentAssign" {
					if pr == nil {
						m.PromptConfirmationBox.Reset()
						return m, m.SetIsPromptConfirmationShown(false)
					}
					role := strings.ToLower(strings.TrimSpace(input))
					kind := agentTaskKind(pr)
					if !agentRoleAllowed(kind, role) {
						m.Ctx.Error = fmt.Errorf("role %q is not available for this task", role)
					} else {
						cmd = m.assignAgentTask(pr, role)
					}
					m.Table.SetRows(m.BuildRows())
					m.PromptConfirmationBox.Reset()
					blinkCmd := m.SetIsPromptConfirmationShown(false)
					return m, tea.Batch(cmd, blinkCmd)
				}
				if action == "agentConfig" {
					providerID := strings.ToLower(strings.TrimSpace(input))
					if strings.HasSuffix(providerID, "-cli") {
						if _, providerErr := provider.NewRegistry().Detect(stdcontext.Background(), providerID, false); providerErr == nil {
							m.Ctx.AgentProviderID = providerID
							cmd = m.runWaitingAgentTasks()
						}
					}
					m.PromptConfirmationBox.Reset()
					blinkCmd := m.SetIsPromptConfirmationShown(false)
					return m, tea.Batch(cmd, blinkCmd)
				}
				sid := tasks.SectionIdentifier{Id: m.Id, Type: SectionType}
				if input == "Y" || input == "y" {
					switch action {
					case "close":
						cmd = tasks.ClosePR(m.Ctx, sid, pr)
					case "reopen":
						cmd = tasks.ReopenPR(m.Ctx, sid, pr)
					case "ready":
						cmd = tasks.PRReady(m.Ctx, sid, pr)
					case "merge":
						cmd = tasks.MergePR(m.Ctx, sid, pr)
					case "update":
						cmd = tasks.UpdatePR(m.Ctx, sid, pr)
					case "approveWorkflows":
						cmd = tasks.ApproveWorkflows(m.Ctx, sid, pr)
					}
				}

				m.PromptConfirmationBox.Reset()
				blinkCmd := m.SetIsPromptConfirmationShown(false)

				return m, tea.Batch(cmd, blinkCmd)
			}

			break
		}

		switch {
		case m.Type == AgentSectionType && key.Matches(msg, keys.PRKeys.AssignAgent):
			m.SetPromptConfirmationAction("assignAgent")
			if queueRow, isQueueRow := m.GetCurrRow().(*AgentQueueRow); isQueueRow && queueRow.Task.Kind == "issue" {
				m.PromptConfirmationBox.SetPrompt("Assign triage or janitor: ")
			} else {
				m.PromptConfirmationBox.SetPrompt("Assign reviewer, sentinel, diagnostics, or janitor: ")
			}
			cmd = m.SetIsPromptConfirmationShown(true)

		case m.Type == AgentSectionType && key.Matches(msg, keys.PRKeys.AgentConfig):
			m.SetPromptConfirmationAction("agentConfig")
			var choices []string
			for _, item := range provider.NewRegistry().ListProviders(stdcontext.Background()) {
				if item.Available && strings.HasSuffix(item.ID, "-cli") {
					choices = append(choices, item.ID)
				}
			}
			if len(choices) == 0 {
				choices = []string{"No provider CLIs detected"}
			}
			m.PromptConfirmationBox.SetPrompt("Available: " + strings.Join(choices, ", ") + " | current: " + m.Ctx.AgentProviderID + " | select ID: ")
			cmd = m.SetIsPromptConfirmationShown(true)

		case m.Type == SectionType && key.Matches(msg, keys.PRKeys.AssignTaskAgent):
			m.SetPromptConfirmationAction("agentAssign")
			m.PromptConfirmationBox.SetPrompt("Assign reviewer, sentinel, diagnostics, or janitor: ")
			cmd = m.SetIsPromptConfirmationShown(true)

		case m.Type == SectionType && key.Matches(msg, keys.PRKeys.AgentTaskConfig):
			m.SetPromptConfirmationAction("agentConfig")
			var choices []string
			for _, item := range provider.NewRegistry().ListProviders(stdcontext.Background()) {
				if item.Available && strings.HasSuffix(item.ID, "-cli") {
					choices = append(choices, item.ID)
				}
			}
			m.PromptConfirmationBox.SetPrompt("Available: " + strings.Join(choices, ", ") + " | current: " + m.Ctx.AgentProviderID + " | select ID: ")
			cmd = m.SetIsPromptConfirmationShown(true)

		case key.Matches(msg, keys.PRKeys.Diff):
			cmd = m.diff()

		case key.Matches(msg, keys.PRKeys.ToggleSmartFiltering):
			before := m.IsFilteredByCurrentRemote

			// If we're filtering by the current repo - we want to remove it
			// If there's no repo filter we want to add the current repo filter.
			if m.HasCurrentRepoNameInConfiguredFilter() || !m.HasRepoNameInConfiguredFilter() {
				m.IsFilteredByCurrentRemote = !before
			}
			log.Debug(
				"toggled smart filtering",
				"before",
				before,
				"after",
				m.IsFilteredByCurrentRemote,
			)
			searchValue := m.GetSearchValue()
			if m.SearchValue != searchValue {
				m.SearchValue = searchValue
				m.SearchBar.SetValue(searchValue)
				m.SetIsSearching(false)
				m.ResetRows()
				return m, tea.Batch(m.FetchNextPageSectionRows()...)
			}

		case key.Matches(msg, keys.PRKeys.Checkout):
			cmd, err = m.checkout()
			if err != nil {
				m.Ctx.Error = err
			}

		case key.Matches(msg, keys.PRKeys.WatchChecks):
			cmd = m.watchChecks()
		}

	case tasks.UpdatePRMsg:
		for i, currPr := range m.Prs {
			if currPr.Primary.Number != msg.PrNumber {
				continue
			}

			if msg.IsClosed != nil {
				if *msg.IsClosed {
					currPr.Primary.State = "CLOSED"
				} else {
					currPr.Primary.State = "OPEN"
				}
			}
			if msg.NewComment != nil {
				currPr.Enriched.Comments.Nodes = append(
					currPr.Enriched.Comments.Nodes, *msg.NewComment)
			}
			if msg.AddedAssignees != nil {
				currPr.Primary.Assignees.Nodes = addAssignees(
					currPr.Primary.Assignees.Nodes, msg.AddedAssignees.Nodes)
			}
			if msg.RemovedAssignees != nil {
				currPr.Primary.Assignees.Nodes = removeAssignees(
					currPr.Primary.Assignees.Nodes, msg.RemovedAssignees.Nodes)
			}
			if msg.Labels != nil {
				currPr.Primary.Labels.Nodes = msg.Labels.Nodes
			}
			if msg.ReadyForReview != nil && *msg.ReadyForReview {
				currPr.Primary.IsDraft = false
			}
			if msg.IsMerged != nil && *msg.IsMerged {
				currPr.Primary.State = "MERGED"
				currPr.Primary.Mergeable = ""
			}
			m.Prs[i] = currPr
			m.SetIsLoading(false)
			m.Table.SetRows(m.BuildRows())
			break
		}

	case SectionPullRequestsFetchedMsg:
		if m.LastFetch.TaskId == msg.TaskId {
			if m.PageInfo != nil {
				m.Prs = append(m.Prs, msg.Prs...)
			} else {
				m.Prs = msg.Prs
			}
			m.TotalCount = msg.TotalCount
			m.PageInfo = &msg.PageInfo
			m.SetIsLoading(false)
			m.Table.SetRows(m.BuildRows())
			m.Table.SetLastUpdated(time.Now())
			m.UpdateTotalItemsCount(m.TotalCount)
		}

	case tasks.AgentRunFinishedMsg:
		if task, ok := m.Ctx.AgentTasks[msg.Key]; ok {
			task.Summary = msg.Summary
			task.Phases = append([]string(nil), msg.Phases...)
			task.UpdatedAt = time.Now()
			if msg.Err != nil {
				task.Status, task.Error = "failed", msg.Err.Error()
				task.CurrentPhase = "Execution failed"
			} else {
				task.Status = msg.Status
				if len(task.Phases) > 0 {
					task.CurrentPhase = task.Phases[len(task.Phases)-1]
				} else {
					task.CurrentPhase = msg.Status
				}
			}
			m.Ctx.AgentTasks[msg.Key] = task
			m.Table.SetRows(m.BuildRows())
			if m.HistoryOnly {
				m.TotalCount = m.NumRows()
			}
		}

	case tasks.AgentTaskUpdatedMsg:
		m.Ctx.AgentTasks[msg.Task.Key] = msg.Task
		m.Table.SetRows(m.BuildRows())
		if m.HistoryOnly {
			m.TotalCount = m.NumRows()
		}

	}

	search, searchCmd := m.SearchBar.Update(msg)
	m.Table.SetRows(m.BuildRows())
	m.SearchBar = search

	prompt, promptCmd := m.PromptConfirmationBox.Update(msg)
	m.PromptConfirmationBox = prompt

	table, tableCmd := m.Table.Update(msg)
	m.Table = table

	return m, tea.Batch(cmd, searchCmd, promptCmd, tableCmd)
}

func (m *Model) GetPromptConfirmation() string {
	if m.IsPromptConfirmationFocused() {
		switch m.GetPromptConfirmationAction() {
		case "agentAssign", "assignAgent", "agentConfig":
			return m.Ctx.Styles.ListViewPort.PagerStyle.Render(m.PromptConfirmationBox.View())
		}
	}
	return m.BaseModel.GetPromptConfirmation()
}

func (m *Model) assignAgentTask(row data.RowData, role string) tea.Cmd {
	if row == nil {
		return nil
	}
	if m.Ctx.AgentTasks == nil {
		m.Ctx.AgentTasks = make(map[string]context.AgentTask)
	}
	kind := agentTaskKind(row)
	role = strings.ToLower(role)
	if !agentRoleAllowed(kind, role) {
		return nil
	}
	key := context.AgentTaskKey(kind, row.GetRepoNameWithOwner(), row.GetNumber())
	task := context.AgentTask{Key: key, SectionID: m.Id, SectionType: m.Type, Kind: kind, Repo: row.GetRepoNameWithOwner(), Number: row.GetNumber(), Title: row.GetTitle(), Role: role, Provider: m.Ctx.AgentProviderID, Status: "queued", UpdatedAt: time.Now()}
	if m.Ctx.AgentProviderID == "" {
		task.Status = "choose CLI"
		task.CurrentPhase = "Waiting for CLI selection"
		task.Phases = []string{"Assignment accepted", task.CurrentPhase}
		m.Ctx.AgentTasks[key] = task
		return nil
	}
	task.Status = "running"
	task.CurrentPhase = "Starting " + task.Provider + " and loading " + kind + " context"
	task.Phases = []string{"Assignment accepted", task.CurrentPhase}
	m.Ctx.AgentTasks[key] = task
	return tasks.RunAgent(stdcontext.Background(), key, task.SectionID, task.SectionType, task.Repo, kind, task.Number, role, m.Ctx.AgentProviderID)
}

func agentTaskKind(row data.RowData) string {
	if queueRow, ok := row.(*AgentQueueRow); ok {
		return queueRow.Task.Kind
	}
	return "pr"
}

func agentRoleAllowed(kind, role string) bool {
	if kind == "issue" {
		return role == "triage" || role == "janitor"
	}
	return role == "reviewer" || role == "sentinel" || role == "diagnostics" || role == "janitor"
}

func (m *Model) runWaitingAgentTasks() tea.Cmd {
	var cmds []tea.Cmd
	for key, task := range m.Ctx.AgentTasks {
		if task.Status != "choose CLI" {
			continue
		}
		task.Status = "running"
		task.Provider = m.Ctx.AgentProviderID
		task.CurrentPhase = "Starting " + task.Provider + " and loading " + task.Kind + " context"
		task.Phases = []string{"Assignment accepted", task.CurrentPhase}
		task.UpdatedAt = time.Now()
		m.Ctx.AgentTasks[key] = task
		cmds = append(cmds, tasks.RunAgent(stdcontext.Background(), key, task.SectionID, task.SectionType, task.Repo, task.Kind, task.Number, task.Role, task.Provider))
	}
	m.Table.SetRows(m.BuildRows())
	return tea.Batch(cmds...)
}

func (m *Model) EnrichPR(data data.EnrichedPullRequestData) {
	for i, currPr := range m.Prs {
		if currPr.Primary.Number != data.Number {
			continue
		}

		m.Prs[i].IsEnriched = true
		m.Prs[i].Enriched = data
	}
}

func GetSectionColumns(
	cfg config.PrsSectionConfig,
	ctx *context.ProgramContext,
) []table.Column {
	dLayout := ctx.Config.Defaults.Layout.Prs
	sLayout := cfg.Layout

	updatedAtLayout := config.MergeColumnConfigs(
		dLayout.UpdatedAt,
		sLayout.UpdatedAt,
	)
	createdAtLayout := config.MergeColumnConfigs(
		dLayout.CreatedAt,
		sLayout.CreatedAt,
	)
	repoLayout := config.MergeColumnConfigs(dLayout.Repo, sLayout.Repo)
	titleLayout := config.MergeColumnConfigs(dLayout.Title, sLayout.Title)
	authorLayout := config.MergeColumnConfigs(dLayout.Author, sLayout.Author)
	assigneesLayout := config.MergeColumnConfigs(
		dLayout.Assignees,
		sLayout.Assignees,
	)
	baseLayout := config.MergeColumnConfigs(dLayout.Base, sLayout.Base)
	numCommentsLayout := config.MergeColumnConfigs(
		dLayout.NumComments,
		sLayout.NumComments,
	)
	reviewStatusLayout := config.MergeColumnConfigs(
		dLayout.ReviewStatus,
		sLayout.ReviewStatus,
	)
	stateLayout := config.MergeColumnConfigs(dLayout.State, sLayout.State)
	ciLayout := config.MergeColumnConfigs(dLayout.Ci, sLayout.Ci)
	labelsLayout := config.MergeColumnConfigs(dLayout.Labels, sLayout.Labels)
	linesLayout := config.MergeColumnConfigs(dLayout.Lines, sLayout.Lines)

	if !ctx.Config.Theme.Ui.Table.Compact {
		return []table.Column{
			{
				Title:  "",
				Width:  utils.IntPtr(3),
				Hidden: stateLayout.Hidden,
			},
			{
				Title:  "Title",
				Grow:   utils.BoolPtr(true),
				Hidden: titleLayout.Hidden,
			},
			{
				Title:  constants.LabelsIcon,
				Width:  labelsLayout.Width,
				Hidden: labelsLayout.Hidden,
			},
			{
				Title:  "Assignees",
				Width:  assigneesLayout.Width,
				Hidden: assigneesLayout.Hidden,
			},
			{
				Title:  "Base",
				Width:  baseLayout.Width,
				Hidden: baseLayout.Hidden,
			},
			{
				Title:  constants.CommentsIcon,
				Width:  utils.IntPtr(4),
				Hidden: numCommentsLayout.Hidden,
			},
			{
				Title:  "󰯢",
				Width:  utils.IntPtr(4),
				Hidden: reviewStatusLayout.Hidden,
			},
			{
				Title:  "",
				Width:  &ctx.Styles.PrSection.CiCellWidth,
				Grow:   new(bool),
				Hidden: ciLayout.Hidden,
			},
			{
				Title:  "",
				Width:  linesLayout.Width,
				Hidden: linesLayout.Hidden,
			},
			{
				Title:  "󱦻",
				Width:  updatedAtLayout.Width,
				Hidden: updatedAtLayout.Hidden,
			},
			{
				Title:  "󱡢",
				Width:  createdAtLayout.Width,
				Hidden: createdAtLayout.Hidden,
			},
			{Title: "Agent", Width: utils.IntPtr(14)},
		}
	}

	return []table.Column{
		{
			Title:  "",
			Width:  utils.IntPtr(3),
			Hidden: stateLayout.Hidden,
		},
		{
			Title:  "",
			Width:  repoLayout.Width,
			Hidden: repoLayout.Hidden,
		},
		{
			Title:  "Title",
			Grow:   utils.BoolPtr(true),
			Hidden: titleLayout.Hidden,
		},
		{
			Title:  "Author",
			Width:  authorLayout.Width,
			Hidden: authorLayout.Hidden,
		},
		{
			Title:  constants.LabelsIcon,
			Width:  labelsLayout.Width,
			Hidden: labelsLayout.Hidden,
		},
		{
			Title:  "Assignees",
			Width:  assigneesLayout.Width,
			Hidden: assigneesLayout.Hidden,
		},
		{
			Title:  "Base",
			Width:  baseLayout.Width,
			Hidden: baseLayout.Hidden,
		},
		{
			Title:  constants.CommentsIcon,
			Width:  utils.IntPtr(4),
			Hidden: numCommentsLayout.Hidden,
		},
		{
			Title:  "󰯢",
			Width:  utils.IntPtr(4),
			Hidden: reviewStatusLayout.Hidden,
		},
		{
			Title:  "",
			Width:  &ctx.Styles.PrSection.CiCellWidth,
			Grow:   new(bool),
			Hidden: ciLayout.Hidden,
		},
		{
			Title:  "",
			Width:  linesLayout.Width,
			Hidden: linesLayout.Hidden,
		},
		{
			Title:  "󱦻",
			Width:  updatedAtLayout.Width,
			Hidden: updatedAtLayout.Hidden,
		},
		{
			Title:  "󱡢",
			Width:  createdAtLayout.Width,
			Hidden: createdAtLayout.Hidden,
		},
		{Title: "Agent", Width: utils.IntPtr(20)},
	}
}

func (m Model) BuildRows() []table.Row {
	if m.Type == AgentSectionType {
		return m.buildAgentRows()
	}

	var rows []table.Row
	currItem := m.Table.GetCurrItem()
	for i, currPr := range m.Prs {
		prModel := prrow.PullRequest{
			Ctx:     m.Ctx,
			Data:    &currPr,
			Columns: m.Table.Columns, ShowAuthorIcon: m.ShowAuthorIcon,
		}
		row := prModel.ToTableRow(currItem == i)
		if task, ok := m.Ctx.AgentTasks[context.AgentTaskKey("pr", currPr.Primary.Repository.NameWithOwner, currPr.Primary.Number)]; ok {
			row = append(row, task.Role+":"+agentTaskStatusLabel(task.Status))
		} else {
			row = append(row, "")
		}
		rows = append(rows, row)
	}

	if rows == nil {
		rows = []table.Row{}
	}

	return rows
}

func (m *Model) NumRows() int {
	if m.Type == AgentSectionType {
		if m.HistoryOnly {
			return len(m.agentHistoryTasks())
		}
		return len(m.Prs) + len(m.agentQueueTasks())
	}
	return len(m.Prs)
}

type SectionPullRequestsFetchedMsg struct {
	Prs        []prrow.Data
	TotalCount int
	PageInfo   data.PageInfo
	TaskId     string
}

func (m *Model) GetCurrRow() data.RowData {
	idx := m.Table.GetCurrItem()
	if idx < 0 {
		return nil
	}
	if m.HistoryOnly {
		tasks := m.agentHistoryTasks()
		if idx >= len(tasks) {
			return nil
		}
		return &AgentQueueRow{Task: tasks[idx]}
	}
	if m.Type == AgentSectionType && idx >= len(m.Prs) {
		issues := m.agentQueueTasks()
		issueIdx := idx - len(m.Prs)
		if issueIdx < 0 || issueIdx >= len(issues) {
			return nil
		}
		row := &AgentQueueRow{Task: issues[issueIdx]}
		return row
	}
	if idx >= len(m.Prs) {
		return nil
	}
	pr := m.Prs[idx]
	return &pr
}

func (m *Model) FetchNextPageSectionRows() []tea.Cmd {
	if m == nil {
		return nil
	}
	if m.HistoryOnly {
		return nil
	}

	// this isn't the first page and there isn't a next page
	if m.PageInfo != nil && !m.PageInfo.HasNextPage {
		return nil
	}

	var cmds []tea.Cmd

	startCursor := time.Now().String()
	if m.PageInfo != nil {
		startCursor = m.PageInfo.StartCursor
	}
	taskId := fmt.Sprintf("fetching_prs_%d_%s", m.Id, startCursor)
	m.LastFetch.TaskId = taskId
	m.LastFetch.At = time.Now()
	task := context.Task{
		Id:        taskId,
		StartText: fmt.Sprintf(`Fetching PRs for "%s"`, m.Config.Title),
		FinishedText: fmt.Sprintf(
			`PRs for "%s" have been fetched`,
			m.Config.Title,
		),
		State: context.TaskStart,
		Error: nil,
	}
	startCmd := m.Ctx.StartTask(task)
	cmds = append(cmds, startCmd)

	fetchCmd := func() tea.Msg {
		limit := m.Config.Limit
		if limit == nil {
			limit = &m.Ctx.Config.Defaults.PrsLimit
		}

		res, err := data.FetchPullRequests(m.GetFilters(), *limit, m.PageInfo)
		if err != nil {
			return constants.TaskFinishedMsg{
				SectionId:   m.Id,
				SectionType: m.Type,
				TaskId:      taskId,
				Err:         err,
			}
		}

		prs := make([]prrow.Data, 0)
		for _, pr := range res.Prs {
			prs = append(prs, prrow.Data{Primary: &pr})
		}
		return constants.TaskFinishedMsg{
			SectionId:   m.Id,
			SectionType: m.Type,
			TaskId:      taskId,
			Msg: SectionPullRequestsFetchedMsg{
				Prs:        prs,
				TotalCount: res.TotalCount,
				PageInfo:   res.PageInfo,
				TaskId:     taskId,
			},
		}
	}
	cmds = append(cmds, fetchCmd)

	cmds = append(cmds, m.SetIsLoading(true))

	return cmds
}

func (m *Model) ResetRows() {
	m.Prs = nil
	m.BaseModel.ResetRows()
}

func InitSections(ctx *context.ProgramContext) []section.Section {
	sections := make([]section.Section, len(ctx.Config.PRSections))
	for i, sectionConfig := range ctx.Config.PRSections {
		sections[i] = new(NewModel(
			i,
			ctx,
			sectionConfig,
			time.Now(),
			time.Now(),
		))
	}

	return sections
}

func InitAgentSections(ctx *context.ProgramContext) []section.Section {
	if ctx.Config == nil || len(ctx.Config.AgentsSections) == 0 {
		return []section.Section{}
	}
	configs := append([]config.AgentsSectionConfig(nil), ctx.Config.AgentsSections...)
	hasHistory := false
	for _, sectionConfig := range configs {
		if strings.EqualFold(sectionConfig.Title, "history") {
			hasHistory = true
			break
		}
	}
	if !hasHistory {
		history := config.AgentsSectionConfig{Title: "History"}
		insertAt := len(configs)
		for i, sectionConfig := range configs {
			if strings.EqualFold(sectionConfig.Title, "working") {
				insertAt = i + 1
				break
			}
		}
		configs = append(configs, config.AgentsSectionConfig{})
		copy(configs[insertAt+1:], configs[insertAt:])
		configs[insertAt] = history
	}
	sections := make([]section.Section, len(configs))
	for i, sectionConfig := range configs {
		sections[i] = new(NewAgentModel(
			i,
			ctx,
			sectionConfig.ToPrsSectionConfig(),
			time.Now(),
			time.Now(),
		))
	}

	return sections
}

func FetchAllSections(
	ctx *context.ProgramContext,
	sections []section.Section,
) tea.Cmd {
	fetchPRsCmds := make([]tea.Cmd, 0)
	prSections := section.ToImplSections[*Model](sections)
	for _, prSection := range prSections {
		if prSection.HistoryOnly {
			prSection.Table.SetRows(prSection.BuildRows())
			prSection.TotalCount = prSection.NumRows()
			continue
		}
		prSection.SetLastUpdated(time.Now())
		prSection.SetCreatedAt(time.Now())
		prSection.PageInfo = nil
		fetchPRsCmds = append(fetchPRsCmds, prSection.FetchNextPageSectionRows()...)
	}

	return tea.Batch(fetchPRsCmds...)
}

func addAssignees(assignees, addedAssignees []data.Assignee) []data.Assignee {
	newAssignees := assignees
	for _, assignee := range addedAssignees {
		if !assigneesContains(newAssignees, assignee) {
			newAssignees = append(newAssignees, assignee)
		}
	}

	return newAssignees
}

func removeAssignees(
	assignees, removedAssignees []data.Assignee,
) []data.Assignee {
	newAssignees := []data.Assignee{}
	for _, assignee := range assignees {
		if !assigneesContains(removedAssignees, assignee) {
			newAssignees = append(newAssignees, assignee)
		}
	}

	return newAssignees
}

func assigneesContains(assignees []data.Assignee, assignee data.Assignee) bool {
	return slices.Contains(assignees, assignee)
}

func (m Model) GetItemSingularForm() string {
	if m.Type == AgentSectionType {
		return "task"
	}
	return "PR"
}

func (m Model) GetItemPluralForm() string {
	if m.Type == AgentSectionType {
		return "tasks"
	}
	return "PRs"
}

func (m Model) GetTotalCount() int {
	return m.TotalCount
}

func (m Model) GetPagerContent() string {
	pagerContent := ""
	timeElapsed := utils.TimeElapsed(m.LastUpdated())
	if timeElapsed == "now" {
		timeElapsed = "just now"
	} else {
		timeElapsed = fmt.Sprintf("~%v ago", timeElapsed)
	}
	if m.TotalCount > 0 {
		pagerContent = fmt.Sprintf(
			"%v Updated %v • %v %v/%v (fetched %v)",
			constants.WaitingIcon,
			timeElapsed,
			m.SingularForm,
			m.Table.GetCurrItem()+1,
			m.TotalCount,
			len(m.Table.Rows),
		)
	}
	pager := m.Ctx.Styles.ListViewPort.PagerStyle.Render(pagerContent)
	return pager
}

func (m *Model) ConfigGetter() config.SectionConfig {
	return m.cfg.ToSectionConfig()
}
