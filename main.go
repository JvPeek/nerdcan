

package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	go listenForCANCtrl()

	messages, err := loadMessages()
	if err != nil {
		fmt.Printf("Error loading messages: %v", err)
		os.Exit(1)
	}

	p := tea.NewProgram(initialModel(messages), tea.WithAltScreen(), tea.WithMouseAllMotion())
	if err := p.Start(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
