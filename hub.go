package websocket

import (
	"net/http"

	log "github.com/Sirupsen/logrus"
)

const (
	//GeneralChannel represents a channel broadcasting to all active connections
	GeneralChannel string = "general"
)

//Hub is broadcasting into and reading from websockets
type Hub interface {
	Broadcast(msg Message, channel string)
	RegisterConnection(writer http.ResponseWriter, req *http.Request, channels []string) (*Connection, error)
	RegisterListener(channel string, l ConnListener)
}

//ConnListener is a Connection listener interface
type ConnListener interface {
	Handle(msg []byte)
	Supports(messageType int) bool
}

//hub maintains a list of active channels with associated websocket connections
type hub struct {
	//channels is a hashmap of hashmaps containing connections
	channels  map[string]map[string]*Connection
	listeners map[string][]ConnListener
	factory   ConnectionFactory
}

//NewHub is a hub constructor
func NewHub() Hub {
	h := hub{make(map[string]map[string]*Connection), make(map[string][]ConnListener), &gorillaFactory{}}
	h.channels[GeneralChannel] = make(map[string]*Connection)
	return Hub(&h)
}

func (h *hub) RegisterConnection(writer http.ResponseWriter, req *http.Request, channels []string) (*Connection, error) {
	var (
		err error
		c   *Connection
	)
	if c, err = h.factory.UpgradeConnection(writer, req, channels); err != nil {
		return c, err
	}
	if log.GetLevel() >= log.InfoLevel {
		log.WithFields(log.Fields{"logger": "ws.hub.register", "connection": c.ID, "remote": c.Remote, "channels": channels}).
			Info("Registering a websocket Connection")
	}
	h.channels[GeneralChannel][c.ID] = c
	for _, ch := range channels {
		if _, ok := h.channels[ch]; !ok {
			h.channels[ch] = make(map[string]*Connection)
		}
		h.channels[ch][c.ID] = c
	}

	go c.WriteLoop()
	go c.ReadLoop()
	go h.listen(c)

	return c, nil
}

func (h *hub) RegisterListener(ch string, l ConnListener) {
	if log.GetLevel() >= log.InfoLevel {
		log.WithFields(log.Fields{"logger": "ws.hub.listener", "channel": ch}).
			Info("Registering a channel listener")
	}
	h.listeners[ch] = append(h.listeners[ch], l)
}

func (h *hub) Broadcast(msg Message, ch string) {
	for _, c := range h.channels[ch] {
		if log.GetLevel() >= log.DebugLevel {
			log.WithFields(log.Fields{"logger": "ws.hub.broadcast", "remote": c.Remote, "id": c.ID, "channel": ch}).
				Debug("Sending message into websocket")
		}
		c.Out <- msg
	}
}

func (h *hub) listen(c *Connection) {
	defer h.remove(c)
	for {
		select {
		case msg, ok := <-c.In:
			if !ok {
				log.WithFields(log.Fields{"logger": "ws.hub.listen", "Connection": c.ID}).
					Info("Input channel is closed; aborting read loop")
				return
			}
			h.callListeners(c, msg)
		case <-c.control:
			// the connection was closed
			return
		}
	}
}

func (h *hub) remove(c *Connection) {
	//delete the connection from broadcast channels
	for _, ch := range c.Channels {
		delete(h.channels[ch], c.ID)
	}
}

func (h *hub) callListeners(c *Connection, msg Message) {
	if log.GetLevel() >= log.DebugLevel {
		if msg.MessageType == TextMessage {
			log.WithFields(log.Fields{"logger": "ws.hub.listen", "connection": c.ID, "msg": string(msg.Payload)}).
				Debug("Calling listeners associated with the connection")
		} else {
			log.WithFields(log.Fields{"logger": "ws.hub.listen", "connection": c.ID}).
				Debug("Calling listeners associated with the connection")
		}
	}
	for _, ch := range c.Channels {
		for _, l := range h.listeners[ch] {
			if l.Supports(msg.MessageType) {
				l.Handle(msg.Payload)
			}
		}
	}
}
