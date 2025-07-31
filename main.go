
package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"go.einride.tech/can"
	"go.einride.tech/can/pkg/socketcan"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// CANMessage holds a CAN frame and its metadata.

type CANMessage struct {
	Frame     can.Frame
	Timestamp time.Time
	CycleTime time.Duration
}

func main() {
	app := tview.NewApplication()

	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true).
		SetChangedFunc(func() {
			app.Draw()
		})
	textView.SetBorder(true).SetTitle("CAN Messages (can0)")

	statusBar := tview.NewTextView().
		SetDynamicColors(true)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(textView, 0, 1, true).
		AddItem(statusBar, 1, 0, false)

	overwriteMode := false
	canMessages := make(map[uint32]CANMessage)

	updateStatus := func() {
		mode := "Log"
		if overwriteMode {
			mode = "Overwrite"
		}
		statusText := fmt.Sprintf("Mode: [yellow]%s[white] (Press 'o' to switch)", mode)
		statusBar.SetText(statusText)
	}

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'o' {
			overwriteMode = !overwriteMode
			updateStatus()
			textView.Clear()
			return nil
		}
		return event
	})

	go func() {
		conn, err := socketcan.DialContext(context.Background(), "can", "can0")
		if err != nil {
			app.QueueUpdateDraw(func() {
				textView.SetText(fmt.Sprintf("Error: %v", err))
			})
			return
		}
		defer conn.Close()

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

			app.QueueUpdateDraw(func() {
				if overwriteMode {
					ids := make([]uint32, 0, len(canMessages))
					for id := range canMessages {
						ids = append(ids, id)
					}
					sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
					textView.Clear()
					for _, id := range ids {
						currentMsg := canMessages[id]
						cycleTimeMs := float64(currentMsg.CycleTime.Nanoseconds()) / 1e6
						fmt.Fprintf(textView, "[green]%s[white] ID: [yellow]%03X[white] DLC: [blue]%d[white] Cycle: [magenta]%-10s[white] Data: [cyan]%X[white]\n",
							currentMsg.Timestamp.Format("15:04:05.000"),
							currentMsg.Frame.ID,
							currentMsg.Frame.Length,
							fmt.Sprintf("%.3fms", cycleTimeMs),
							currentMsg.Frame.Data,
						)
					}
				} else {
					cycleTimeMs := float64(msg.CycleTime.Nanoseconds()) / 1e6
					fmt.Fprintf(textView, "[green]%s[white] ID: [yellow]%03X[white] DLC: [blue]%d[white] Cycle: [magenta]%-10s[white] Data: [cyan]%X[white]\n",
						msg.Timestamp.Format("15:04:05.000"),
						msg.Frame.ID,
						msg.Frame.Length,
						fmt.Sprintf("%.3fms", cycleTimeMs),
						msg.Frame.Data,
					)
				}
			})
		}
	}()

	updateStatus()
	app.SetRoot(flex, true).SetFocus(flex)

	if err := app.Run(); err != nil {
		log.Fatalf("Error running application: %v", err)
	}
}
