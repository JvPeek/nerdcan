package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"go.einride.tech/can"
	"go.einride.tech/can/pkg/socketcan"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model is the main model for the Bubble Tea application.

type Model struct {
	table         table.Model
	overwriteMode bool
	canMessages   map[uint32]CANMessage
	width, height int
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

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if err := p.Start(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

func initialModel() Model {
	columns := []table.Column{
		{Title: "Timestamp", Width: 12},
		{Title: "ID", Width: 6},
		{Title: "DLC", Width: 5},
		{Title: "Cycle Time", Width: 15},
		{Title: "Data", Width: 25},
	}

	t := table.New(
				table.WithColumns(columns),
				table.WithFocused(true),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240")).BorderBottom(true).Bold(false)
	s.Selected = s.Selected.Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Bold(false)
	t.SetStyles(s)

	return Model{table: t, canMessages: make(map[uint32]CANMessage)}
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
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(m.width)
		m.table.SetHeight(m.height - 5) // 5 for header and footer
	case CANMessage:
		m.canMessages[msg.Frame.ID] = msg
		var rows []table.Row
		if m.overwriteMode {
			ids := make([]uint32, 0, len(m.canMessages))
			for id := range m.canMessages {
				ids = append(ids, id)
			}
			sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
			for _, id := range ids {
				rows = append(rows, canMessageToRow(m.canMessages[id]))
			}
		} else {
			rows = m.table.Rows()
			rows = append(rows, canMessageToRow(msg))
		}
		m.table.SetRows(rows)
		return m, waitForCANMessage
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		Padding(0, 1).
		Render("nerdcan")

	mode := "Log"
	if m.overwriteMode {
		mode = "Overwrite"
	}
	status := fmt.Sprintf("Mode: %s | Messages: %d", mode, len(m.canMessages))

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Padding(0, 1).
		Render(fmt.Sprintf("q: quit | o: switch mode | %s", status))

	return lipgloss.JoinVertical(lipgloss.Left, header, m.table.View(), footer)
}

func canMessageToRow(msg CANMessage) table.Row {
	cycleTimeMs := float64(msg.CycleTime.Nanoseconds()) / 1e6
	return table.Row{
		msg.Timestamp.Format("15:04:05.000"),
		fmt.Sprintf("0x%03X", msg.Frame.ID),
		fmt.Sprintf("%d", msg.Frame.Length),
		fmt.Sprintf("%.3fms", cycleTimeMs),
		fmt.Sprintf("%X", msg.Frame.Data),
	}
}

func listenForCANCtrl() {
	conn, err := socketcan.DialContext(context.Background(), "can", "can0")
	if err != nil {
		// Handle error - maybe send an error message to the tea.Model
		return
	}
	defer conn.Close()

	canMessages := make(map[uint32]CANMessage)
	rx := socketcan.NewReceiver(conn)
	for rx.Receive() {
		frame := rx.Frame()
		receivedTime := time.Now()

		var cycleTime time.Duration
		if lastMsg, ok := canMessages[frame.ID]; ok {
			cycleTime = receivedTime.Sub(lastMsg.Timestamp)
		}

		msg := CANMessage{
			Frame:     frame,
			Timestamp: receivedTime,
			CycleTime: cycleTime,
		}
		canMessages[frame.ID] = msg
		canMsgCh <- msg
	}
}

func waitForCANMessage() tea.Msg {
	return <-canMsgCh
}