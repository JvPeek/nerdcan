package main

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// form represents the new message creation form.

type form struct {
	inputs  []textinput.Model
	focused int
}

func newForm() form {
	inputs := make([]textinput.Model, 11)
	for i := range inputs {
		inputs[i] = textinput.New()
		inputs[i].Prompt = ""
	}

	inputs[0].Placeholder = "1A"
	inputs[0].CharLimit = 3
	inputs[0].Width = 5

	inputs[1].Placeholder = "8"
	inputs[1].CharLimit = 1
	inputs[1].Width = 3

	inputs[2].Placeholder = "100"
	inputs[2].CharLimit = 5
	inputs[2].Width = 7

	for i := 3; i < 11; i++ {
		inputs[i].CharLimit = 2
		inputs[i].Width = 3
	}

	return form{inputs: inputs, focused: -1}
}

func updateForm(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down":
			m.form.inputs[m.form.focused].Blur()
			m.form.focused = (m.form.focused + 1) % len(m.form.inputs)
			m.form.inputs[m.form.focused].Focus()
		case "shift+tab", "up":
			m.form.inputs[m.form.focused].Blur()
			m.form.focused--
			if m.form.focused < 0 {
				m.form.focused = len(m.form.inputs) - 1
			}
			m.form.inputs[m.form.focused].Focus()
		case "enter":
			id, _ := strconv.ParseUint(m.form.inputs[0].Value(), 16, 32)
			dlc, _ := strconv.ParseUint(m.form.inputs[1].Value(), 10, 8)
			cycle, _ := time.ParseDuration(m.form.inputs[2].Value() + "ms")
			data := make([]byte, dlc)
			for i := 0; i < int(dlc); i++ {
				b, _ := hex.DecodeString(m.form.inputs[3+i].Value())
				if len(b) > 0 {
					data[i] = b[0]
				}
			}
			m.sendMessages = append(m.sendMessages, &SendMessage{ID: uint32(id), DLC: uint8(dlc), CycleTime: cycle, Data: data, done: make(chan bool)})
			m.updateSendTable()
			m.form.focused = -1
			return m, nil
		case "esc":
			m.form.focused = -1
			return m, nil
		}
	}

	var cmds []tea.Cmd
	for i := range m.form.inputs {
		var c tea.Cmd
		m.form.inputs[i], c = m.form.inputs[i].Update(msg)
		cmds = append(cmds, c)
	}

	return m, tea.Batch(cmds...)
}

func (f form) View(m Model) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ID: 0x%s\n", f.inputs[0].View())
	fmt.Fprintf(&b, "DLC: %s\n", f.inputs[1].View())
	fmt.Fprintf(&b, "Cycle: %s ms\n", f.inputs[2].View())

	dataFields := []string{}
	for i := 3; i < 11; i++ {
		dataFields = append(dataFields, f.inputs[i].View())
	}
	fmt.Fprintf(&b, "Data: %s\n", strings.Join(dataFields, " "))

	popup := popupStyle.Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup)
}
