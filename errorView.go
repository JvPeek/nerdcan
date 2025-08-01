package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) renderErrorView() string {
	// The popup's width is set to m.width - 6.
	// The popupStyle has horizontal padding of 2 on each side (4 total) and
	// a border that takes up 1 character on each side (2 total).
	// The total horizontal space consumed by the popup's chrome is 4 + 2 = 6.
	// So, the inner content width for the table is the popup's width minus the chrome.
	// (m.width - 6) - 6 = m.width - 12.
	// The main view has a padding of 1 on each side, so we subtract 2 more.
	tableContentWidth := m.width - 14

	columns := []table.Column{
		{Title: "Level", Width: 8},
		{Title: "Source", Width: 25},
		{Title: "Message", Width: tableContentWidth - 8 - 25 - 2}, // Remaining width after Level and Source, and 2 internal separators
	}

	var rows []table.Row
	for _, log := range GetLogs() {
		rows = append(rows, table.Row{
			log.Level.String(),
			fmt.Sprintf("%s:%d", log.File, log.Line),
			log.Message,
		})
	}

	logTable := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithWidth(tableContentWidth), // Set the total width of the table
		table.WithHeight(m.height - popupStyle.GetVerticalPadding() - popupStyle.GetVerticalBorderSize() - 2), // Adjust height dynamically
	)

	logStyles := table.DefaultStyles()
	logStyles.Header = logStyles.Header.BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240")).BorderBottom(true).Bold(false)
	logStyles.Selected = logStyles.Selected.Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Bold(false)
	logTable.SetStyles(logStyles)

	// The popup itself is m.width - 6 wide
	helpBox := popupStyle.Width(m.width - 6).Render(logTable.View())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, helpBox)
}
