package websocket

import "encoding/json"

//JSONMessage is a message containing JSON payload
func JSONMessage(title string, body string, errMsg string) ([]byte, error) {
	msg := Message{Title: title, Body: body, Error: errMsg}
	var JSON []byte
	var err error
	if JSON, err = json.Marshal(msg); err != nil {
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
