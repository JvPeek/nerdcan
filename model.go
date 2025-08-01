package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

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

type InfoPanelTickMsg time.Time

func infoPanelTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return InfoPanelTickMsg(t)
	})
}

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
	showInfo      bool
	showLogs      bool
	showDetail    bool
	infoPanel     info
	logTable      table.Model
	detailPanel   detailModel
	canInterface  string
}

type detailModel struct {
	message CANMessage
	visible bool
	width   int
	height  int
}

func newDetailModel() detailModel {
	return detailModel{
		visible: false,
	}
}

func (dm detailModel) Init() tea.Cmd {
	return nil
}

func (dm detailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		dm.width = msg.Width
		dm.height = msg.Height
	}
	return dm, nil
}

func (dm detailModel) View() string {
	if !dm.visible {
		return ""
	}

	contentBuilder := strings.Builder{}
	contentBuilder.WriteString(detailViewHeaderStyle.Render("Message Details") + "\n\n")

	// ID, DLC, Cycle
	infoLine := fmt.Sprintf("ID: 0x%03X | DLC: %d | Cycle: %.3fms", dm.message.Frame.ID, dm.message.Frame.Length, float64(dm.message.CycleTime.Nanoseconds())/1e6)
	contentBuilder.WriteString(infoLine + "\n")

	// Data in Hex
	hexData := make([]string, len(dm.message.Frame.Data))
	for i, b := range dm.message.Frame.Data {
		hexData[i] = fmt.Sprintf("%02X", b)
	}
	hexLine := fmt.Sprintf("Data (Hex): %s", strings.Join(hexData, " "))
	contentBuilder.WriteString(hexLine + "\n")

	// Data in Binary
	binData := make([]string, len(dm.message.Frame.Data))
	for i, b := range dm.message.Frame.Data {
		binData[i] = fmt.Sprintf("%08b", b)
	}

	// Calculate available content width for binary data (popup is 100% width, but with a small margin)
	actualPopupWidth := dm.width - 2 // Give it a 1-char margin on each side
	availableContentWidth := actualPopupWidth - (popupStyle.GetHorizontalPadding() * 2) - (popupStyle.GetHorizontalBorderSize() * 2)

	// Calculate the width if binary data is on one line
	binaryOneLineContent := fmt.Sprintf("Data (Binary): %s", strings.Join(binData, " "))
	requiredBinOneLineWidth := lipgloss.Width(binaryOneLineContent)

	if requiredBinOneLineWidth <= availableContentWidth {
		contentBuilder.WriteString(binaryOneLineContent + "\n")
	} else {
		contentBuilder.WriteString("Data (Binary):\n")
		for _, b := range binData {
			contentBuilder.WriteString(fmt.Sprintf("  %s\n", b))
		}
	}

	detailBox := popupStyle.Width(actualPopupWidth).Height(dm.height - 4).Render(contentBuilder.String())

	return lipgloss.Place(dm.width, dm.height, lipgloss.Center, lipgloss.Center, detailBox)
}

func initialModel(messages []*SendMessage, canInterface string) Model {
	receiveTable := newReceiveTable()
	sendTable := newSendTable()

	model := Model{
		receiveTable:  receiveTable,
		sendTable:     sendTable,
		canMessages:   make(map[uint32]CANMessage),
		filteredIDs:   make(map[uint32]struct{}),
		overwriteMode: true, // Default to overwrite mode
		focus:         FocusBottom,
		form:          newForm("", "", "", ""),
		showHelp:      false,
		showInfo:      false,
		infoPanel:     newInfo(),
		sendMessages:  messages,
		logTable:      table.New(table.WithColumns([]table.Column{})), // Initialize with empty columns
		canInterface:  canInterface,
		detailPanel:   newDetailModel(),
	}

	model.updateSendTable()
	model.sendTable.Focus()
	model.receiveTable.Blur()
	return model
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
			case "L":
				m.showLogs = !m.showLogs
				if m.showLogs {
					newErrorFlagMutex.Lock()
					newErrorFlag = false
					newErrorFlagMutex.Unlock()
				}
				return m, nil
			case "esc":
				if m.showHelp {
					m.showHelp = false
					return m, nil
				}
				if m.showLogs {
					m.showLogs = false
					return m, nil
				}
				if m.showInfo {
					m.showInfo = false
					// Stop the bus load monitor goroutine
					close(m.infoPanel.stopChan)
					// Re-initialize the stop channel for the next time it's opened
					m.infoPanel.stopChan = make(chan struct{})
					return m, nil
				}
				if m.showDetail {
					m.showDetail = false
					return m, nil
				}
				// If no popups are open, clear messages and stop cyclic sending
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
				var id, dlc, cycleTime, data string
				if m.focus == FocusTop {
					selectedRow := m.receiveTable.SelectedRow()
					if selectedRow != nil {
						id = strings.TrimPrefix(selectedRow[1], "0x") // ID
						dlc = selectedRow[2] // DLC
						// Remove "ms" from cycle time string
						cycleTime = strings.TrimSuffix(selectedRow[3], "ms") // Cycle Time
						data = selectedRow[4] // Data
					}
				}
				m.form = newForm(id, dlc, cycleTime, data) // Always start with a fresh form
				m.form.focused = 0
				for i := range m.form.inputs {
					m.form.inputs[i].Blur()
				}
				m.form.inputs[0].Focus()
				return m, nil
			case "e":
				if m.focus == FocusBottom {
					selectedRow := m.sendTable.SelectedRow()
					if selectedRow != nil {
						// UUID is the first column (index 0)
						id := strings.TrimPrefix(selectedRow[2], "0x") // ID is the third column (index 2)
						dlc := selectedRow[3] // DLC is the fourth column (index 3)
						cycleTime := strings.TrimSuffix(selectedRow[4], "ms") // Cycle Time is the fifth column (index 4)
						data := selectedRow[5] // Data is the sixth column (index 5)

						m.form = newForm(id, dlc, cycleTime, data) // Populate form for editing
						m.form.editingUUID = selectedRow[0] // Store UUID for update
						m.form.focused = 0
						for i := range m.form.inputs {
							m.form.inputs[i].Blur()
						}
						m.form.inputs[0].Focus()
						return m, nil
					}
				}
				return m, nil
			case "d":
				if m.focus == FocusTop { // Only allow 'd' in receive panel
					m.showDetail = !m.showDetail
					if m.showDetail {
						selectedRow := m.receiveTable.SelectedRow()
						if selectedRow != nil {
							idStr := selectedRow[1] // ID is the second column
							id, err := strconv.ParseUint(strings.TrimPrefix(idStr, "0x"), 16, 32)
							if err == nil {
								if msg, ok := m.canMessages[uint32(id)]; ok {
									m.detailPanel.message = msg
									m.detailPanel.visible = true
								}
							}
						}
					} else {
						m.detailPanel.visible = false
					}
				}
				return m, nil
			case "backspace", "delete": // New keybinding for deleting send messages
				if m.focus == FocusBottom {
					selectedRow := m.sendTable.SelectedRow()
					if selectedRow != nil {
						selectedUUID := selectedRow[0] // UUID is the first column
						newSendMessages := []*SendMessage{}
						for _, msg := range m.sendMessages {
							if msg.UUID.String() != selectedUUID {
								newSendMessages = append(newSendMessages, msg)
							} else {
								if msg.Sending {
									msg.done <- true // Stop cyclic sending if active
								}
							}
						}
						m.sendMessages = newSendMessages
						m.updateSendTable()
						saveMessages(m.sendMessages)
					}
				}
				return m, nil
			case "ctrl+s":
				saveMessages(m.sendMessages)
				return m, nil
			case "ctrl+l":
				loadedMessages, err := loadMessages()
				if err == nil {
					m.sendMessages = loadedMessages
					m.updateSendTable()
				}
				return m, nil
			case "ctrl+d":
				for _, msg := range m.sendMessages {
					if msg.Sending {
						msg.done <- true // Stop cyclic sending if active
					}
				}
				m.sendMessages = []*SendMessage{} // Clear all messages
				m.updateSendTable()
				saveMessages(m.sendMessages)
				return m, nil
			case "i":
				m.showInfo = !m.showInfo
				if m.showInfo {
					go m.infoPanel.startBusLoadMonitor() // Start the bus load monitor goroutine
					m.infoPanel.updateInfo()
					return m, infoPanelTickCmd()
				} else {
					// Stop the bus load monitor goroutine
					close(m.infoPanel.stopChan)
					// Re-initialize the stop channel for the next time it's opened
					m.infoPanel.stopChan = make(chan struct{})
				}
				return m, nil
			case " ": // Spacebar to send message
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
								go sendCyclic(msg, m.canInterface)
							}
						} else {
							go sendOnce(msg, m.canInterface)
						}
						m.updateSendTable()
					}
				}
				return m, nil
			}
		}
	case InfoPanelTickMsg:
		if m.showInfo {
			m.infoPanel.updateInfo()
			return m, infoPanelTickCmd()
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()
		// Pass the window size message to the detail panel
		var updatedDetailModel tea.Model
		var detailCmd tea.Cmd
		updatedDetailModel, detailCmd = m.detailPanel.Update(msg)
		m.detailPanel = updatedDetailModel.(detailModel)
		cmd = tea.Batch(cmd, detailCmd)
	case CANMessage:
		if msg.Direction == "RX" && m.canMessages[msg.Frame.ID].SentByApp {
			return m, waitForCANMessage // Ignore echoed message
		}

		// Create a new CANMessage to store, copying relevant fields
		msgToStore := msg

		prevMsg, exists := m.canMessages[msg.Frame.ID]
		if exists {
			// Only calculate cycle time for received messages
			if !msgToStore.SentByApp {
				msgToStore.CycleTime = msg.Timestamp.Sub(prevMsg.Timestamp)
			}
			// Preserve SentByApp status if it was previously true
			if prevMsg.SentByApp {
				msgToStore.SentByApp = true
			}
		}
		m.canMessages[msg.Frame.ID] = msgToStore

		// Update detail panel if visible and message ID matches
		if m.showDetail && m.detailPanel.visible && m.detailPanel.message.Frame.ID == msg.Frame.ID {
			m.detailPanel.message = msgToStore
		}

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
			// In log mode, add message to table if it's a new received message
			// or a message sent by the app. Filter out echoed messages.
			shouldAdd := true
			if !msgToStore.SentByApp { // If it's a received message
				if existingMsg, ok := m.canMessages[msg.Frame.ID]; ok && existingMsg.SentByApp {
					// If there's an existing message with the same ID that was sent by the app,
					// and this is a received message, then it's an echo. Don't add it.
					shouldAdd = false
				}
			}

			if shouldAdd {
				rows = m.receiveTable.Rows()
				rows = append(rows, m.canMessageToRow(msgToStore))
				m.receiveTable.GotoBottom()
			}
		}
		m.receiveTable.SetRows(rows)
		return m, waitForCANMessage
	}

	if m.form.focused > -1 {
		return updateForm(m, msg)
	} else if m.showLogs {
		m.logTable, cmd = m.logTable.Update(msg)
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

	if m.showLogs {
		return m.renderErrorView()
	}

	if m.showInfo {
		return m.infoPanel.View(m)
	}

	if m.showDetail {
		return m.detailPanel.View()
	}

	header := headerStyle.Width(m.width).Render("NerdCAN")
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
	maxWidth := 0

	// Helper function to add a line and update maxWidth
	addLine := func(s string) {
		helpBuilder.WriteString(s + "\n")
		currentWidth := lipgloss.Width(s)
		if currentWidth > maxWidth {
			maxWidth = currentWidth
		}
	}

	addLine(lipgloss.NewStyle().Bold(true).Render(NerdCANASCII))
	addLine("")
	addLine(" q: quit")
	addLine(" ?: toggle help")
	addLine(" i: toggle info panel")
	addLine("")

	addLine(lipgloss.NewStyle().Bold(true).Render("RECEIVE PANE"))
	addLine(" o: toggle mode (overwrite/log)")
	addLine(" f: cycle filter mode")
	addLine(" F: add/remove selected ID to filter")
	addLine(" esc: clear all received messages")
	addLine(" tab: switch focus")
	addLine("")

	addLine(lipgloss.NewStyle().Bold(true).Render("SEND PANE"))
	addLine(" n: create new message")
	addLine(" e: edit selected message")
	addLine(" d: delete selected message")
	addLine(" space: send selected message")
	addLine(" ctrl+s: save messages")
	addLine(" ctrl+l: load messages")
		addLine(" ctrl+d: clear all messages")
	addLine(" i: toggle info panel")
	addLine(" esc: stop all cyclic messages")
	addLine(" tab: switch focus")

	// Add padding for the border
	helpBoxWidth := maxWidth + popupStyle.GetHorizontalPadding() + popupStyle.GetHorizontalBorderSize()

	helpBox := popupStyle.Width(helpBoxWidth).Render(helpBuilder.String())

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

	statusRight := "? for help"
	if m.hasNewErrorLogs() && !m.showLogs {
		statusRight = "Error, press Shift+L"
	}

	statusLeftWidth := lipgloss.Width(statusLeft)
	statusRightWidth := lipgloss.Width(statusRight)

	padding := m.width - statusLeftWidth - statusRightWidth
	if padding < 0 {
		padding = 0
	}

	finalStatus := lipgloss.JoinHorizontal(lipgloss.Left, statusLeft, strings.Repeat(" ", padding), statusRight)

	return statusStyle.Width(m.width).Render(finalStatus)
}

func (m *Model) hasNewErrorLogs() bool {
	newErrorFlagMutex.Lock()
	defer newErrorFlagMutex.Unlock()
	return newErrorFlag
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

	directionIcon := ""
	if msg.Direction == "TX" {
		directionIcon = "▲"
	} else {
		directionIcon = "▼"
	}

	return table.Row{
		fmt.Sprintf("%s%s", indicator, directionIcon),
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
	dataBytes := make([]string, len(msg.Data))
	for i, b := range msg.Data {
		dataBytes[i] = fmt.Sprintf("%02X", b)
	}
	dataStr := strings.Join(dataBytes, " ")
	return table.Row{
		msg.UUID.String(),
		indicator,
		fmt.Sprintf("0x%03X", msg.ID),
		fmt.Sprintf("%d", msg.DLC),
		func() string {
			if msg.CycleTime == 0 {
				return ""
			}
			return fmt.Sprintf("%v", msg.CycleTime)
		}(),
		dataStr,
		msg.TriggerType,
	}
}