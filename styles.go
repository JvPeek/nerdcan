package main

import "github.com/charmbracelet/lipgloss"

var (
	headerStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1")).Padding(0, 1).Align(lipgloss.Center)
	statusStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Background(lipgloss.Color("0"))
	popupStyle          = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).BorderForeground(lipgloss.Color("4"))
	activeBorderStyle   = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("1"))  // Red
	inactiveBorderStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("8")) // Dark Gray

	rxStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // Green
	txStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1")) // Red
	detailViewHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1")).Padding(0, 1).Align(lipgloss.Center)
)
