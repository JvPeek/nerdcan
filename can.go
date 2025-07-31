package main

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.einride.tech/can"
	"go.einride.tech/can/pkg/socketcan"
	tea "github.com/charmbracelet/bubbletea"
)

// CANMessage holds a CAN frame and its metadata.

type CANMessage struct {
	Frame     can.Frame
	Timestamp time.Time
	CycleTime time.Duration
	Direction string // "RX" or "TX"
	SentByApp bool   // True if this message was sent by the application
}

// SendMessage holds a custom CAN message to be sent.

type SendMessage struct {
	UUID      uuid.UUID
	ID        uint32
	DLC       uint8
	CycleTime time.Duration
	Data      []byte
	Sending   bool
	TriggerType string // "manual" or "timer"
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
			canMsgCh <- CANMessage{Frame: receiver.Frame(), Timestamp: time.Now(), Direction: "RX", SentByApp: false}
		}
	}
}

func waitForCANMessage() tea.Msg {
	return <-canMsgCh
}

func sendOnce(msg *SendMessage) {
	msg.TriggerType = "manual"
	conn, err := socketcan.DialContext(context.Background(), "can", "can0")
	if err != nil {
		return
	}
	defer conn.Close()

	tx := socketcan.NewTransmitter(conn)
	frame := can.Frame{ID: msg.ID, Length: msg.DLC, Data: can.Data(msg.Data)}
	_ = tx.TransmitFrame(context.Background(), frame)
	canMsgCh <- CANMessage{Frame: frame, Timestamp: time.Now(), Direction: "TX", SentByApp: true, CycleTime: 0}
}

func sendCyclic(msg *SendMessage) {
	msg.TriggerType = "timer"
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
			canMsgCh <- CANMessage{Frame: frame, Timestamp: time.Now(), Direction: "TX", SentByApp: true, CycleTime: msg.CycleTime}
		case <-msg.done:
			msg.ticker.Stop()
			return
		}
	}
}

