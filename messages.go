package websocket

import (
	"encoding/json"
	"fmt"

	log "github.com/Sirupsen/logrus"
)

//JSONMessage is a message containing JSON payload
func JSONMessage(title string, body string, errMsg string) ([]byte, error) {
	msg := Message{Title: title, Body: body, Error: errMsg}
	var JSON []byte
	var err error
	if JSON, err = json.Marshal(msg); err != nil {
		log.WithFields(log.Fields{"logger": "ws.messages", "method": "JSONMessage", "title": title, "message": fmt.Sprintf("%+v", msg)}).
			WithError(err).Error("Could not marshal the message into JSON.")
		return nil, err
	}
	return JSON, nil
}

//Message is a generic message type
type Message struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Error string `json:"error,omitempty"`
}
