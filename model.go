package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	FilterModeOff = iota
	FilterModeWhitelist
	FilterModeBlacklist
)

const (
	FocusTop = iota
	FocusBottom
)

const infoPanelWidth = 30

// Model is the main model for the Bubble Tea application.

type Model struct {
	receiveTable  table.Model
	sendTable     table.Model
	overwriteMode bool
	canMessages   map[uint32]CANMessage
	sendMessages  []*SendMessage
	width, height int
	filterMode    int
	filteredIDs   map[uint32]struct{}
	focus         int
	form          form
	showHelp      bool
}

func initialModel() Model {
	receiveTable := newReceiveTable()
	sendTable := newSendTable()

	return Model{
		receiveTable:  receiveTable,
		sendTable:     sendTable,
		canMessages:   make(map[uint32]CANMessage),
		filteredIDs:   make(map[uint32]struct{}),
		overwriteMode: true, // Default to overwrite mode
		focus:         FocusTop,
		form:          newForm(),
		showHelp:      false,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(waitForCANMessage, tea.EnterAltScreen)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.form.focused > -1 {
			return updateForm(m, msg)
		} else {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "?":
				m.showHelp = !m.showHelp
				return m, nil
			case "o":
				m.overwriteMode = !m.overwriteMode
				return m, nil
			case "f":
				m.filterMode = (m.filterMode + 1) % 3
				m.receiveTable.SetRows([]table.Row{}) // Clear table on filter toggle
				return m, nil
			case "F":
				selectedRow := m.receiveTable.SelectedRow()
				if selectedRow != nil {
					idStr := selectedRow[1] // ID is the second column now
					id, err := strconv.ParseUint(strings.TrimPrefix(idStr, "0x"), 16, 32)
					if err == nil {
						if _, exists := m.filteredIDs[uint32(id)]; exists {
							delete(m.filteredIDs, uint32(id))
						} else {
							m.filteredIDs[uint32(id)] = struct{}{}
						}
					}
				}
				return m, nil
			case "esc":
				m.canMessages = make(map[uint32]CANMessage)
				m.receiveTable.SetRows([]table.Row{})
				for _, msg := range m.sendMessages {
					if msg.Sending {
						msg.done <- true
						msg.Sending = false
					}
				}
				m.updateSendTable()
				return m, nil
			case "tab":
				if m.form.focused == -1 {
					m.focus = (m.focus + 1) % 2
					if m.focus == FocusTop {
						m.receiveTable.Focus()
						m.sendTable.Blur()
					} else {
						m.receiveTable.Blur()
						m.sendTable.Focus()
					}
				}
				return m, nil
			case "n":
				m.form.focused = 0
				for i := range m.form.inputs {
					m.form.inputs[i].Blur()
				}
				m.form.inputs[0].Focus()
				return m, nil
			case " ":
				if m.focus == FocusBottom {
					selectedRow := m.sendTable.SelectedRow()
					if selectedRow != nil {
						msg := m.sendMessages[m.sendTable.Cursor()]
						if msg.CycleTime > 0 {
							if msg.Sending {
								msg.done <- true
								msg.Sending = false
							} else {
								msg.Sending = true
								go sendCyclic(msg)
							}
						} else {
							go sendOnce(msg)
						}
						m.updateSendTable()
					}
				}
				return m, nil
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()
	case CANMessage:
		prevMsg, exists := m.canMessages[msg.Frame.ID]
		if exists {
			msg.CycleTime = msg.Timestamp.Sub(prevMsg.Timestamp)
		}
		m.canMessages[msg.Frame.ID] = msg

		switch m.filterMode {
		case FilterModeWhitelist:
			if _, ok := m.filteredIDs[msg.Frame.ID]; !ok {
				return m, waitForCANMessage
			}
		case FilterModeBlacklist:
			if _, ok := m.filteredIDs[msg.Frame.ID]; ok {
				return m, waitForCANMessage
			}
		}

		var rows []table.Row
		if m.overwriteMode {
			ids := make([]uint32, 0, len(m.canMessages))
			for id := range m.canMessages {
				add := true
				switch m.filterMode {
				case FilterModeWhitelist:
					if _, ok := m.filteredIDs[id]; !ok {
						add = false
					}
				case FilterModeBlacklist:
					if _, ok := m.filteredIDs[id]; ok {
						add = false
					}
				}
				if add {
					ids = append(ids, id)
				}
			}
			sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
			for _, id := range ids {
				rows = append(rows, m.canMessageToRow(m.canMessages[id]))
			}
		} else {
			rows = m.receiveTable.Rows()
			rows = append(rows, m.canMessageToRow(msg))
			m.receiveTable.GotoBottom()
		}
		m.receiveTable.SetRows(rows)
		return m, waitForCANMessage
	}

	if m.form.focused > -1 {
		return updateForm(m, msg)
	} else if m.focus == FocusTop {
		m.receiveTable, cmd = m.receiveTable.Update(msg)
	} else {
		m.sendTable, cmd = m.sendTable.Update(msg)
	}
	return m, cmd
}

func (m *Model) updateLayout() {
	mainViewHeight := m.height - 2 // For header and footer
	topPaneHeight := mainViewHeight / 2
	bottomPaneHeight := mainViewHeight - topPaneHeight
	tableWidth := m.width - 2

	m.receiveTable.SetWidth(tableWidth)
	m.receiveTable.SetHeight(topPaneHeight - 2)
	m.sendTable.SetWidth(tableWidth)
	m.sendTable.SetHeight(bottomPaneHeight - 2)
}

func (m Model) View() string {
	if m.form.focused > -1 {
		return m.form.View(m)
	}

	if m.showHelp {
		return m.renderHelpView()
	}

	header := headerStyle.Render("nerdcan")
	statusBar := m.renderStatusBar()

	topPaneStyle := inactiveBorderStyle
	bottomPaneStyle := inactiveBorderStyle

	if m.focus == FocusTop {
		topPaneStyle = activeBorderStyle
	} else {
		bottomPaneStyle = activeBorderStyle
	}

	topPane := topPaneStyle.Width(m.width - 2).Render(m.receiveTable.View())
	bottomPane := bottomPaneStyle.Width(m.width - 2).Render(m.sendTable.View())

	mainView := lipgloss.JoinVertical(lipgloss.Left, topPane, bottomPane)

	return lipgloss.JoinVertical(lipgloss.Left, header, mainView, statusBar)
}

func (m *Model) renderHelpView() string {
	var helpBuilder strings.Builder
	helpBuilder.WriteString(lipgloss.NewStyle().Bold(true).Render("GENERAL") + "\n")
	helpBuilder.WriteString(" q: quit\n")
	helpBuilder.WriteString(" ?: toggle help\n\n")

	helpBuilder.WriteString(lipgloss.NewStyle().Bold(true).Render("RECEIVE PANE") + "\n")
	helpBuilder.WriteString(" o: toggle mode (overwrite/log)\n")
	helpBuilder.WriteString(" f: cycle filter mode\n")
	helpBuilder.WriteString(" F: add/remove selected ID to filter\n")
	helpBuilder.WriteString(" esc: clear all received messages\n")
	helpBuilder.WriteString(" tab: switch focus\n\n")

	helpBuilder.WriteString(lipgloss.NewStyle().Bold(true).Render("SEND PANE") + "\n")
	helpBuilder.WriteString(" n: create new message\n")
	helpBuilder.WriteString(" space: send selected message\n")
	helpBuilder.WriteString(" esc: stop all cyclic messages\n")
	helpBuilder.WriteString(" tab: switch focus\n")

	helpBox := popupStyle.Width(40).Render(helpBuilder.String())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, helpBox)
}

func (m *Model) renderStatusBar() string {
	mode := "Log"
	if m.overwriteMode {
		mode = "Overwrite"
	}

	var filterStatus string
	switch m.filterMode {
	case FilterModeOff:
		filterStatus = "Off"
	case FilterModeWhitelist:
		filterStatus = "Whitelist"
	case FilterModeBlacklist:
		filterStatus = "Blacklist"
	}

	statusLeft := fmt.Sprintf(" %s | %d msgs | Filter: %s", mode, len(m.canMessages), filterStatus)
	statusRight := "? for help "

	statusLeftWidth := lipgloss.Width(statusLeft)
	statusRightWidth := lipgloss.Width(statusRight)

	padding := m.width - statusLeftWidth - statusRightWidth
	if padding < 0 {
		padding = 0
	}

	finalStatus := lipgloss.JoinHorizontal(lipgloss.Left, statusLeft, strings.Repeat(" ", padding), statusRight)

	return statusStyle.Width(m.width).Render(finalStatus)
}



func (m *Model) canMessageToRow(msg CANMessage) table.Row {
	cycleTimeMs := float64(msg.CycleTime.Nanoseconds()) / 1e6
	dataBytes := make([]string, len(msg.Frame.Data))
	for i, b := range msg.Frame.Data {
		dataBytes[i] = fmt.Sprintf("%02X", b)
	}
	dataStr := strings.Join(dataBytes, " ")

	indicator := "  "
	if _, ok := m.filteredIDs[msg.Frame.ID]; ok {
		indicator = "• "
	}

	return table.Row{
		indicator,
		fmt.Sprintf("0x%03X", msg.Frame.ID),
		fmt.Sprintf("%d", msg.Frame.Length),
		fmt.Sprintf("%.3fms", cycleTimeMs),
		dataStr,
		msg.Timestamp.Format("15:04:05.000"),
	}
}

func (m *Model) updateSendTable() {
	rows := []table.Row{}
	for _, msg := range m.sendMessages {
		rows = append(rows, m.sendMessageToRow(msg))
	}
	m.sendTable.SetRows(rows)
}

func (m *Model) sendMessageToRow(msg *SendMessage) table.Row {
	indicator := "  "
	if msg.Sending {
		indicator = "> "
	}
	return table.Row{
		indicator,
		fmt.Sprintf("0x%03X", msg.ID),
		fmt.Sprintf("%d", msg.DLC),
		fmt.Sprintf("%v", msg.CycleTime),
		fmt.Sprintf("%X", msg.Data),
	}
}
