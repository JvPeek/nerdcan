package main

import (
	"context"
	"time"

	"go.einride.tech/can"
	"go.einride.tech/can/pkg/socketcan"
	tea "github.com/charmbracelet/bubbletea"
)

// CANMessage holds a CAN frame and its metadata.

type CANMessage struct {
	Frame     can.Frame
	Timestamp time.Time
	CycleTime time.Duration
}

// SendMessage holds a custom CAN message to be sent.

type SendMessage struct {
	ID        uint32
	DLC       uint8
	CycleTime time.Duration
	Data      []byte
	Sending   bool
	ticker    *time.Ticker
	done      chan bool
}

// A chan to receive CAN messages.
var canMsgCh = make(chan CANMessage)

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

func sendOnce(msg *SendMessage) {
	conn, err := socketcan.DialContext(context.Background(), "can", "can0")
	if err != nil {
		return
	}
	defer conn.Close()

	tx := socketcan.NewTransmitter(conn)
	frame := can.Frame{ID: msg.ID, Length: msg.DLC, Data: can.Data(msg.Data)}
	_ = tx.TransmitFrame(context.Background(), frame)
}

func sendCyclic(msg *SendMessage) {
	conn, err := socketcan.DialContext(context.Background(), "can", "can0")
	if err != nil {
		return
	}
	defer conn.Close()

	tx := socketcan.NewTransmitter(conn)
	frame := can.Frame{ID: msg.ID, Length: msg.DLC, Data: can.Data(msg.Data)}

	msg.ticker = time.NewTicker(msg.CycleTime)
	for {
		select {
		case <-msg.ticker.C:
			_ = tx.TransmitFrame(context.Background(), frame)
		case <-msg.done:
			msg.ticker.Stop()
			return
		}
	}
}
