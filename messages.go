package websocket

import (
	"encoding/json"

	log "github.com/sirupsen/logrus"
)

/*
TypedJSONMessage contains a type/title and a JSON encoded payload;
The payload is thus encoded twice: once as JSON and the second time inside the
JSON message. In this way we can parse the message, determine payload type and
parse it into appropriate struct.
*/
type TypedJSONMessage struct {
	Title   string `json:"title"`
	Payload string `json:"body"`
}

//Unwrap parses JSON payload into a struct
func (m *TypedJSONMessage) Unwrap(in interface{}) error {
	return json.Unmarshal([]byte(m.Payload), in)
}

/*
EncodeMessage encodes arbitrary payload into TypedJSONMessage of a given type
*/
func EncodeMessage(title string, payload interface{}) []byte {
	var (
		err error
		b   []byte
	)
	if b, err = json.Marshal(payload); err != nil {
		log.WithFields(log.Fields{"logger": "ws.message"}).
			WithError(err).Error("Could not marshal content to json")
		return nil
	}
	msg := &TypedJSONMessage{
		Title:   title,
		Payload: string(b),
	}
	if b, err = json.Marshal(msg); err != nil {
		log.WithFields(log.Fields{"logger": "ws.message"}).
			WithError(err).Error("Could not produce json message")
	}
	return b
}
