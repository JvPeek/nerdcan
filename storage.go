package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"time"

	"github.com/google/uuid"
)

const messagesFileName = "messages.json"

// sendMessagesJSON is a helper struct for JSON marshaling/unmarshaling
// because time.Duration and uuid.UUID don't directly support it.
type sendMessageJSON struct {
	UUID        string `json:"uuid"`
	ID          uint32 `json:"id"`
	DLC         uint8  `json:"dlc"`
	CycleTimeMs int64  `json:"cycle_time_ms"`
	Data        []byte `json:"data"`
}

func saveMessages(messages []*SendMessage) {
	var jsonMessages []sendMessageJSON
	for _, msg := range messages {
		jsonMessages = append(jsonMessages, sendMessageJSON{
			UUID:        msg.UUID.String(),
			ID:          msg.ID,
			DLC:         msg.DLC,
			CycleTimeMs: msg.CycleTime.Milliseconds(),
			Data:        msg.Data,
		})
	}

	data, err := json.MarshalIndent(jsonMessages, "", "  ")
	if err != nil {
		Log(ERROR, "Error marshaling messages to JSON: %v", err)
		return
	}

	err = ioutil.WriteFile(messagesFileName, data, 0644)
	if err != nil {
		Log(ERROR, "Error writing messages to file: %v", err)
	}
}

func loadMessages() ([]*SendMessage, error) {
	data, err := ioutil.ReadFile(messagesFileName)
	if err != nil {
		if os.IsNotExist(err) {
			return []*SendMessage{}, nil // Return empty slice if file doesn't exist
		}
		return nil, err
	}

	var jsonMessages []sendMessageJSON
	err = json.Unmarshal(data, &jsonMessages)
	if err != nil {
		return nil, err
	}

	var messages []*SendMessage
	for _, jsonMsg := range jsonMessages {
		u, err := uuid.Parse(jsonMsg.UUID)
		if err != nil {
			Log(WARNING, "Skipping message with invalid UUID: %s", jsonMsg.UUID)
			continue
		}
		messages = append(messages, &SendMessage{
			UUID:        u,
			ID:          jsonMsg.ID,
			DLC:         jsonMsg.DLC,
			CycleTime:   time.Duration(jsonMsg.CycleTimeMs) * time.Millisecond,
			Data:        jsonMsg.Data,
			done:        make(chan bool), // Re-initialize channel
		})
	}

	return messages, nil
}
