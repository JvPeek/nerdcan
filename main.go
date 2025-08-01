

package main

import (
	"flag"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	canInterface := flag.String("d", "can0", "CAN interface to use")
	flag.Parse()

	go listenForCANCtrl(*canInterface)

	Log(INFO, "NerdCAN started successfully")

	messages, err := loadMessages()
	if err != nil {
		Log(ERROR, "Error loading messages: %v", err)
	}

	p := tea.NewProgram(initialModel(messages, *canInterface), tea.WithAltScreen(), tea.WithMouseAllMotion())
	if err := p.Start(); err != nil {
		Log(CRISIS, "main.p.Start", "Alas, there's been an error: %v", err)
	}
}
