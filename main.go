
package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.einride.tech/can"
	"go.einride.tech/can/pkg/socketcan"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	FilterModeOff = iota
	FilterModeWhitelist
	FilterModeBlacklist
)

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Padding(0, 1)
	footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Background(lipgloss.Color("235"))
	infoStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
)

const infoPanelWidth = 30

// Model is the main model for the Bubble Tea application.

type Model struct {
	table         table.Model
	overwriteMode bool
	canMessages   map[uint32]CANMessage
	width, height int
	filterMode    int
	filteredIDs   map[uint32]struct{}
}

// CANMessage holds a CAN frame and its metadata.

type CANMessage struct {
	Frame     can.Frame
	Timestamp time.Time
	CycleTime time.Duration
}

// A chan to receive CAN messages.
var canMsgCh = make(chan CANMessage)

func main() {
	go listenForCANCtrl()

	p := tea.NewProgram(initialModel(), tea.WithAltScreen(), tea.WithMouseAllMotion())
	if err := p.Start(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

func initialModel() Model {
	columns := []table.Column{
		{Title: "", Width: 2},
		{Title: "ID", Width: 8},
		{Title: "DLC", Width: 4},
		{Title: "Cycle Time", Width: 14},
		{Title: "Data", Width: 24},
		{Title: "Timestamp", Width: 12},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240")).BorderBottom(true).Bold(false)
	s.Selected = s.Selected.Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Bold(false)
	t.SetStyles(s)

	return Model{
		table:         t,
		canMessages:   make(map[uint32]CANMessage),
		filteredIDs:   make(map[uint32]struct{}),
		overwriteMode: true, // Default to overwrite mode
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(waitForCANMessage, tea.EnterAltScreen)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "o":
			m.overwriteMode = !m.overwriteMode
			return m, nil
		case "f":
			m.filterMode = (m.filterMode + 1) % 3
			m.table.SetRows([]table.Row{}) // Clear table on filter toggle
			return m, nil
		case "F":
			selectedRow := m.table.SelectedRow()
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
			m.table.SetRows([]table.Row{})
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(m.width - infoPanelWidth - 4) // account for padding/borders
		m.table.SetHeight(m.height - 3)                // 3 for header and footer
		infoStyle.Height(m.height - 3)
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
			rows = m.table.Rows()
			rows = append(rows, m.canMessageToRow(msg))
			m.table.GotoBottom()
		}
		m.table.SetRows(rows)
		return m, waitForCANMessage
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	header := headerStyle.Render("nerdcan")
	footerText := "q: quit | o: mode | f: filter mode | F: add/remove from filter | esc: clear"
	footer := footerStyle.Width(m.width).Render(footerText)

	infoPanel := infoStyle.Render(m.renderInfoPanel())

	mainView := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.table.View(),
		infoPanel,
	)

	finalView := lipgloss.JoinVertical(lipgloss.Left, header, mainView, footer)

	return finalView
}

func (m *Model) renderInfoPanel() string {
	mode := "Log"
	if m.overwriteMode {
		mode = "Overwrite"
	}

	var filterStatus string
	switch m.filterMode {
	case FilterModeOff:
		filterStatus = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("Off")
	case FilterModeWhitelist:
		filterStatus = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("Whitelist")
	case FilterModeBlacklist:
		filterStatus = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render("Blacklist")
	}

	var idsBuilder strings.Builder
	if len(m.filteredIDs) > 0 {
		ids := make([]string, 0, len(m.filteredIDs))
		for id := range m.filteredIDs {
			ids = append(ids, fmt.Sprintf("0x%X", id))
		}
		sort.Strings(ids)
		idsBuilder.WriteString(strings.Join(ids, ", "))
	} else {
		idsBuilder.WriteString("None")
	}

	info := []string{
		lipgloss.NewStyle().Bold(true).Render("STATUS"),
		fmt.Sprintf("Mode: %s", mode),
		fmt.Sprintf("Messages: %d", len(m.canMessages)),
		"",
		lipgloss.NewStyle().Bold(true).Render("FILTER"),
		fmt.Sprintf("Mode: %s", filterStatus),
		fmt.Sprintf("IDs: %s", idsBuilder.String()),
	}

	return strings.Join(info, "\n")
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

func listenForCANCtrl() {
	conn, err := socketcan.DialContext(context.Background(), "can", "can0")
	if err != nil {
		// In a real app, you'd send this error to the model to display.
		return
	}
	defer conn.Close()

	receiver := socketcan.NewReceiver(conn)
	for {
		if receiver.Receive() {
			canMsgCh <- CANMessage{Frame: receiver.Frame(), Timestamp: time.Now()}
		}
	}
}

func waitForCANMessage() tea.Msg {
	return <-canMsgCh
}
