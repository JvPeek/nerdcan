package main

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

func newReceiveTable() table.Model {
	receiveColumns := []table.Column{
		{Title: "", Width: 2},
		{Title: "ID", Width: 8},
		{Title: "DLC", Width: 4},
		{Title: "Cycle Time", Width: 14},
		{Title: "Data", Width: 24},
		{Title: "Timestamp", Width: 12},
	}

	receiveTable := table.New(
		table.WithColumns(receiveColumns),
		table.WithFocused(true),
	)

	receiveStyles := table.DefaultStyles()
	receiveStyles.Header = receiveStyles.Header.BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240")).BorderBottom(true).Bold(false)
	receiveStyles.Selected = receiveStyles.Selected.Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Bold(false)
	receiveTable.SetStyles(receiveStyles)
	return receiveTable
}

func newSendTable() table.Model {
	sendColumns := []table.Column{
		{Title: "UUID", Width: 0},
		{Title: "", Width: 3},
		{Title: "ID", Width: 8},
		{Title: "DLC", Width: 4},
		{Title: "Cycle Time", Width: 14},
		{Title: "Data", Width: 24},
	}

	sendTable := table.New(
		table.WithColumns(sendColumns),
	)

	sendStyles := table.DefaultStyles()
	sendStyles.Header = sendStyles.Header.BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240")).BorderBottom(true).Bold(false)
	sendStyles.Selected = sendStyles.Selected.Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Bold(false)
	sendTable.SetStyles(sendStyles)
	return sendTable
}
