package main

import "github.com/charmbracelet/lipgloss"

var (
	headerStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Padding(0, 1)
	footerStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Background(lipgloss.Color("235"))
	infoStyle           = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
	popupStyle          = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).BorderForeground(lipgloss.Color("63"))
	activeBorderStyle   = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("2"))  // Green
	inactiveBorderStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240")) // Gray
)
