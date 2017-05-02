package websocket

import (
	"sync"
	"time"

	uuid "github.com/satori/go.uuid"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
)

const (
	//writeTimeout is the time allowed to write a message to the peer.
	writeTimeout = 10 * time.Second

	//pingTimeout is the time allowed until the ping times out.
	pingTimeout = 10 * time.Second

	// Send pings to peer with this period.
	pingPeriod = 7 * time.Second

	// Maximum message size allowed from peer.
	maxMessageSize = 32768
)

//Websocket connection close causes
const (
	CloseNormalClosure           = 1000
	CloseGoingAway               = 1001
	CloseProtocolError           = 1002
	CloseUnsupportedData         = 1003
	CloseNoStatusReceived        = 1005
	CloseAbnormalClosure         = 1006
	CloseInvalidFramePayloadData = 1007
	ClosePolicyViolation         = 1008
	CloseMessageTooBig           = 1009
	CloseMandatoryExtension      = 1010
	CloseInternalServerErr       = 1011
	CloseServiceRestart          = 1012
	CloseTryAgainLater           = 1013
	CloseTLSHandshake            = 1015
)

// The message types are defined in RFC 6455, section 11.8.
const (
	// TextMessage denotes a text data message. The text message payload is
	// interpreted as UTF-8 encoded text data.
	TextMessage = 1

	// BinaryMessage denotes a binary data message.
	BinaryMessage = 2

	// CloseMessage denotes a close control message. The optional message
	// payload contains a numeric code and text. Use the FormatCloseMessage
	// function to format a close message payload.
	CloseMessage = 8

	// PingMessage denotes a ping control message. The optional message payload
	// is UTF-8 encoded text.
	PingMessage = 9

	// PongMessage denotes a ping control message. The optional message payload
	// is UTF-8 encoded text.
	PongMessage = 10
)

/*
Message is used to pass binary and text messages to the connection through a common channel
*/
type Message struct {
	MessageType int
	Payload     []byte
}

//DisconnectOrigin indicates who initialized connection close
type DisconnectOrigin bool

//origin values
const (
	Self DisconnectOrigin = false
	Peer DisconnectOrigin = true
)

//connection state
const (
	StateClosed int8 = iota
	StateOpen
	StateClosing
)

//rawWebsocket is an interface wrapper over *websocket.connection
type rawWebsocket interface {
	SetWriteDeadline(t time.Time) error
	Close() error
	SetReadLimit(limit int64)
	SetReadDeadline(t time.Time) error
	SetCloseHandler(h func(code int, text string) error)
	SetPongHandler(h func(appData string) error)
	SetPingHandler(h func(appData string) error)
	WriteMessage(messageType int, data []byte) error
	ReadMessage() (messageType int, p []byte, err error)
}

//Connection represents a single client websocket connection
type Connection struct {
	ID             string
	ws             rawWebsocket
	control        chan bool
	pong           chan bool
	In             chan Message
	Out            chan Message
	shutdown       sync.Mutex
	state          int8
	OnClose        func(code int, text string, origin DisconnectOrigin) error //close handler
	Remote         string
	Channels       []string
	WriteTimeout   time.Duration
	PingTimeout    time.Duration
	PingPeriod     time.Duration
	MaxMessageSize int64
}

//WrapConnection wraps gorilla websocket into a Connection
func WrapConnection(ws rawWebsocket, remote string) *Connection {
	return newConnection(ws, remote, nil)
}

//newConnection is the connection constructor
func newConnection(ws rawWebsocket, remote string, channels []string) *Connection {
	c := &Connection{
		ID:             uuid.NewV4().String(),
		ws:             ws,
		control:        make(chan bool, 1),
		In:             make(chan Message, 2048),
		Out:            make(chan Message, 16),
		pong:           make(chan bool, 1),
		shutdown:       sync.Mutex{},
		state:          StateOpen,
		Remote:         remote,
		Channels:       channels,
		WriteTimeout:   writeTimeout,
		PingTimeout:    pingTimeout,
		PingPeriod:     pingPeriod,
		MaxMessageSize: maxMessageSize,
	}
	ws.SetPongHandler(c.pongHandler)
	return c
}

//Close is a shorthand for CloseWithReason with 'no status received' status
func (c *Connection) Close() {
	c.CloseWithReason(CloseNoStatusReceived, "")
}

//CloseWithCode code is a shorthand for CloseWithReason where the reason string is not provided
func (c *Connection) CloseWithCode(code int) {
	c.CloseWithReason(code, "")
}

func (c *Connection) handleCloseMessage(code int, reason string) {
	// if we initialized close handshake we are responsible for closing the net connection
	if c.state == StateClosing {
		defer c.ws.Close()
		//we run the close listener if specified
		if c.OnClose != nil {
			defer c.OnClose(code, reason, Self)
		}
	} else {
		/*
			If the peer initialized the handshake, close message response is sent
			by the onClose handler and the peer will close the network connection.
			We should stop processing messages.
		*/
		c.state = StateClosing
		if c.OnClose != nil {
			defer c.OnClose(code, reason, Peer)
		}
	}
	c.CloseWithReason(code, reason)
}

//CloseWithReason initializes ws close handshake with a given close code and reason
func (c *Connection) CloseWithReason(code int, reason string) {
	// we make sure that Close doesn't get called twice
	c.shutdown.Lock()
	defer c.shutdown.Unlock()

	switch c.state {
	case StateClosed:
		//if the connection is already closed there is nothing left to do
		return
	case StateOpen:
		/*
			if the connection is open we initialize the close handshake, stop the write loop
			and set the connection state to 'closing'
		*/
		log.WithFields(log.Fields{"logger": "ws.connection.close", "conn": c.ID, "remote": c.Remote}).
			Info("Initializing close handshake")
		if err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason)); err != nil {
			log.WithFields(log.Fields{"logger": "ws.connection", "method": "close"}).
				WithError(err).Warn("Error writing close message to the connection")
		}
		c.state = StateClosing
	case StateClosing:
		/*
			If the connection is closing, we stop the read loop and remaining channels and
			we set connection state to 'closed'
		*/
		log.WithFields(log.Fields{"logger": "ws.connection.close", "conn": c.ID, "remote": c.Remote}).
			Info("Closing channels")
		c.control <- true
		close(c.control)
		// sleep to let the control channel trigger loops closing
		time.Sleep(100 * time.Millisecond)
		close(c.In)
		close(c.Out)
		close(c.pong)
		c.state = StateClosed
	}
}

//ReadMessage is a proxy to underlying gorilla websocket read
func (c *Connection) ReadMessage() (int, []byte, error) {
	return c.ws.ReadMessage()
}

//ReadLoop dispatches messages from the websocket to the output channel.
func (c *Connection) ReadLoop() {
	c.ws.SetReadLimit(c.MaxMessageSize)

	var (
		mt  int
		msg []byte
		err error
	)
	for {
		if mt, msg, err = c.ws.ReadMessage(); err != nil {
			if e, ok := err.(*websocket.CloseError); ok {
				log.WithFields(log.Fields{"logger": "ws.connection.read"}).
					Info("Websocket close message received")

				if websocket.IsUnexpectedCloseError(err, CloseGoingAway, CloseNormalClosure, CloseNoStatusReceived) {
					log.WithFields(log.Fields{"logger": "ws.connection.read"}).
						WithError(err).Error("Unexpected close message code received")
				}
				c.handleCloseMessage(e.Code, e.Text)
				return
			}
			log.WithFields(log.Fields{"logger": "ws.connection.read"}).
				WithError(err).Error("Unexpected websocket error received")
			c.CloseWithReason(CloseAbnormalClosure, "Unexpected websocket error received")
			return
		}
		if log.GetLevel() >= log.DebugLevel {
			log.WithFields(log.Fields{"logger": "ws.connection.read", "conn": c.ID, "remote": c.Remote}).
				Debug("Message received")
		}
		c.In <- Message{MessageType: mt, Payload: msg}
	}
}

//WriteMessage writes a message with the given message type and payload.
func (c *Connection) WriteMessage(mt int, payload []byte) error {
	var err error
	if err = c.ws.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return err
	}
	return c.ws.WriteMessage(mt, payload)
}

//WriteLoop pumps messages from the output channel to the websocket connection.
func (c *Connection) WriteLoop() {
	//ping ticker
	ticker := time.NewTicker(c.PingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()
	for {
		select {
		case msg, ok := <-c.Out:
			if !ok {
				log.WithFields(log.Fields{"logger": "ws.connection.write", "conn": c.ID, "message": msg}).
					Warn("Output channel is closed")
				continue
			}
			if log.GetLevel() >= log.DebugLevel {
				log.WithFields(log.Fields{"logger": "ws.connection.write", "remote": c.Remote, "conn": c.ID}).
					Debug("Sending message into websocket")
			}
			if err := c.WriteMessage(msg.MessageType, msg.Payload); err != nil {
				log.WithFields(log.Fields{"logger": "ws.connection.write", "conn": c.ID, "message": msg}).
					WithError(err).Error("Error sending text message into the websocket")
				c.CloseWithReason(CloseAbnormalClosure, "Error sending text message into the websocket")
				return
			}
		case <-ticker.C:
			log.WithFields(log.Fields{"logger": "ws.connection.write", "conn": c.ID}).Debug("Writing ping")
			if err := c.WriteMessage(PingMessage, []byte{}); err != nil {
				log.WithFields(log.Fields{"logger": "ws.connection.write", "conn": c.ID}).
					WithError(err).Error("Error sending ping into the socket")
				c.CloseWithReason(CloseAbnormalClosure, "Error sending ping into the socket")
				return
			}
			go c.waitForPong()
		case <-c.control:
			ticker.Stop()
			return
		}
	}
}

func (c *Connection) waitForPong() {
	log.WithFields(log.Fields{"logger": "ws.connection.pong", "conn": c.ID}).Debug("Setting pong timer")
	select {
	case <-time.After(c.PingTimeout):
		log.WithFields(log.Fields{"logger": "ws.connection.pong", "conn": c.ID}).Debug("Ping timed out, closing connection")
		c.CloseWithCode(CloseNormalClosure)
	case <-(*c).pong:
		if log.GetLevel() >= log.DebugLevel {
			log.WithFields(log.Fields{"logger": "ws.connection.pong", "conn": c.ID}).Debug("Pong timeout cancelled")
		}
	}
}

func (c *Connection) pongHandler(appData string) error {
	c.pong <- true
	return nil
}
