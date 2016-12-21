package websocket

import (
	"net/http"

	log "github.com/Sirupsen/logrus"
)

//ConnectionType is used to specify the type of registered websocket connection
type ConnectionType byte

const (
	//GeneralChannel represents a channel broadcasting to all active connections
	GeneralChannel string         = "general"
	none           ConnectionType = 0x00
	//ReadOnly identifies a read only websocket connection
	ReadOnly ConnectionType = 0x01
	//WriteOnly identifies a write only websocket connection
	WriteOnly ConnectionType = 0x02
	//Duplex identifies a read and write websocket connection
	Duplex ConnectionType = 0x03
)

//Hub is broadcasting into and reading from websockets
type Hub interface {
	Broadcast(msg []byte, channel string)
	RegisterConnection(writer http.ResponseWriter, req *http.Request, channels []string, t ConnectionType) (string, error)
	RegisterListener(channel string, l ConnListener)
}

//ConnListener is a Connection listener interface
type ConnListener interface {
	Handle(msg interface{})
}

//hub maintains a list of active channels with associated websocket connections
type hub struct {
	//channels is a hashmap of hashmaps containing connections
	channels  map[string]map[string]Connection
	listeners map[string][]ConnListener
	factory   ConnectionFactory
}

//NewHub is a hub constructor
func NewHub() Hub {
	h := hub{make(map[string]map[string]Connection), make(map[string][]ConnListener), &gorillaFactory{}}
	h.channels[GeneralChannel] = make(map[string]Connection)
	return Hub(&h)
}

func (h *hub) RegisterConnection(writer http.ResponseWriter, req *http.Request, channels []string, t ConnectionType) (string, error) {
	var err error
	var c Connection
	if c, err = h.factory.UpgradeConnection(writer, req, channels); err != nil {
		return "", err
	}
	ID := c.ID()
	if log.GetLevel() >= log.InfoLevel {
		log.WithFields(log.Fields{"logger": "ws.hub", "method": "RegisterConnection", "conneciton": ID, "host": c.Host(), "channels": channels}).
			Info("Registering a websocket Connection")
	}
	h.channels[GeneralChannel][ID] = c
	for _, ch := range channels {
		if _, ok := h.channels[ch]; !ok {
			h.channels[ch] = make(map[string]Connection)
		}
		h.channels[ch][ID] = c
	}
	if t&0x02 == 0x02 {
		go c.WriteLoop(c.Out())
	}
	if t&0x01 == 0x01 {
		go c.ReadLoop()
		go h.readFrom(c)
	}
	return ID, nil
}

func (h *hub) RegisterListener(ch string, l ConnListener) {
	if log.GetLevel() >= log.InfoLevel {
		log.WithFields(log.Fields{"logger": "ws.hub", "method": "RegisterListener", "channel": ch}).
			Info("Registering a Connection listener")
	}
	h.listeners[ch] = append(h.listeners[ch], l)
}

func (h *hub) Broadcast(msg []byte, channel string) {
	for _, c := range h.channels[channel] {
		if log.GetLevel() >= log.DebugLevel {
			log.WithFields(log.Fields{"logger": "ws.hub", "method": "Broadcast", "host": c.Host(), "id": c.ID(), "channel": channel}).
				Debug("Sending message into websocket")
		}
		c.Out() <- msg
	}
}

func (h *hub) readFrom(c Connection) {
	defer h.cleanup(c)
	in, intxt := c.In()
	ctrl := c.Control()
	for {
		select {
		case msg, ok := <-intxt:
			if !ok {
				if log.GetLevel() >= log.InfoLevel {
					log.WithFields(log.Fields{"logger": "ws.hub", "method": "readFrom", "Connection": c.ID()}).
						Info("Text input channel is closed; aborting read loop")
				}
				return
			}
			if log.GetLevel() >= log.DebugLevel {
				log.WithFields(log.Fields{"logger": "ws.hub", "method": "readFrom", "Connection": c.ID(), "msg": msg}).
					Debug("Calling listeners and publishing message to the event bus")
			}
			h.callListeners(c, msg)
		case msg, ok := <-in:
			if !ok {
				if log.GetLevel() >= log.InfoLevel {
					log.WithFields(log.Fields{"logger": "ws.hub", "method": "readFrom", "Connection": c.ID()}).
						Info("Binary input channel is closed; aborting read loop")
				}
				return
			}
			if log.GetLevel() >= log.DebugLevel {
				log.WithFields(log.Fields{"logger": "ws.hub", "method": "readFrom", "Connection": c.ID(), "msg": msg}).
					Debug("Calling listeners and publishing message to the event bus")
			}
			h.callListeners(c, msg)
		case <-ctrl:
			// the Connection was closed either internally or externally
			return
		}
	}
}

func (h *hub) cleanup(c Connection) {
	c.CloseWithCode(CloseNormalClosure)
	close(c.Out())
	//delete the Connection from broadcast channels
	for _, ch := range c.Channels() {
		delete(h.channels[ch], c.ID())
	}
}

func (h *hub) callListeners(c Connection, msg interface{}) {
	for _, ch := range c.Channels() {
		for _, l := range h.listeners[ch] {
			l.Handle(msg)
		}
	}
}
