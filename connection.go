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
	maxMessageSize = 2048
)

//Connection is a wrapper over raw websocket that exposes read and write channels
//and defines read and write loops
type Connection interface {
	ID() string
	ReadLoop()
	WriteLoop(<-chan []byte)
	Close()
	Control() chan bool
	In() (chan []byte, chan string)
	Out() chan []byte
	Host() string
	Channels() []string
}

//rawWebsocket is an interface wrapper over *websocket.connection
type rawWebsocket interface {
	SetWriteDeadline(t time.Time) error
	Close() error
	SetReadLimit(limit int64)
	SetReadDeadline(t time.Time) error
	SetPongHandler(h func(appData string) error)
	SetPingHandler(h func(appData string) error)
	WriteMessage(messageType int, data []byte) error
	ReadMessage() (messageType int, p []byte, err error)
}

//conn represents a single client websocket connection
type conn struct {
	cid            string
	ws             rawWebsocket
	control        chan bool
	pong           chan bool
	in             chan []byte
	intxt          chan string
	out            chan []byte
	shutdown       sync.Mutex
	closed         bool
	host           string
	channels       []string
	writeTimeout   time.Duration
	pingTimeout    time.Duration
	pingPeriod     time.Duration
	maxMessageSize int
}

//newConnection is the connection constructor
func newConnection(ws rawWebsocket, out chan []byte, host string, channels []string) Connection {
	c := &conn{
		cid:            uuid.NewV4().String(),
		ws:             ws,
		control:        make(chan bool, 1),
		pong:           make(chan bool, 1),
		in:             make(chan []byte, 2048),
		intxt:          make(chan string, 1),
		out:            out,
		shutdown:       sync.Mutex{},
		closed:         false,
		host:           host,
		channels:       channels,
		writeTimeout:   writeTimeout,
		pingTimeout:    pingTimeout,
		pingPeriod:     pingPeriod,
		maxMessageSize: maxMessageSize,
	}
	c.ws.SetPongHandler(c.pongHandler)
	return Connection(c)
}

//ID returns connection's ID
func (c *conn) ID() string {
	return c.cid
}

//Control returns connection's control channel
func (c *conn) Control() chan bool {
	return c.control
}

//Out returns connection's output channel
func (c *conn) Out() chan []byte {
	return c.out
}

//In returns connection's input channels
func (c *conn) In() (chan []byte, chan string) {
	return c.in, c.intxt
}

//Host returns connection's peer hostname
func (c *conn) Host() string {
	return c.host
}

//Channels returns channels this connection is attached to
func (c *conn) Channels() []string {
	return c.channels
}

//Close cleans up gracefully
func (c *conn) Close() {
	// we make sure that Close doesn't get called twice as this might cause writing to a closed channel
	c.shutdown.Lock()
	defer c.shutdown.Unlock()
	//if the connection is already closed there is nothing left to do
	if c.closed {
		return
	}
	log.WithFields(log.Fields{"logger": "ws.connection", "method": "Close", "host": c.host}).
		Info("Closing connection")
	// notify the outside world that the connection is closing
	c.control <- true
	var err error
	// write the close message to the peer
	if err = c.write(websocket.CloseMessage, []byte{}); err != nil {
		log.WithFields(log.Fields{"logger": "ws.connection", "method": "close"}).
			WithError(err).Warn("Error writing close message to the connection")
	}
	// close the websocket
	if err = c.ws.Close(); err != nil {
		log.WithFields(log.Fields{"logger": "ws.connection", "method": "close"}).
			WithError(err).Warn("Error closing websocket connection")
	}
	// close channels
	close(c.in)
	close(c.intxt)
	close(c.pong)
	close(c.control)
	c.closed = true
}

//ReadLoop dispatches messages from the websocket to the output channel.
func (c *conn) ReadLoop() {
	defer func() {
		c.Close()
	}()
	c.ws.SetReadLimit(maxMessageSize)

	var mt int
	var message []byte
	var err error
	for {
		if mt, message, err = c.ws.ReadMessage(); err != nil {
			log.WithFields(log.Fields{"logger": "ws.connection", "method": "ReadLoop"}).
				WithError(err).Error("Websocket read error encountered")
			break
		}
		switch mt {
		case websocket.BinaryMessage:
			if log.GetLevel() >= log.DebugLevel {
				log.WithFields(log.Fields{"logger": "ws.connection", "method": "ReadLoop", "host": c.host}).
					Debug("Binary message received")
			}
			c.in <- message
		case websocket.TextMessage:
			if log.GetLevel() >= log.DebugLevel {
				log.WithFields(log.Fields{"logger": "ws.connection", "method": "ReadLoop", "message": string(message)}).
					Debug("Text message received")
			}
			c.intxt <- string(message[:])
		}
	}
}

//write writes a message with the given message type and payload.
func (c *conn) write(mt int, payload []byte) error {
	var err error
	if err = c.ws.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return err
	}
	return c.ws.WriteMessage(mt, payload)
}

//WriteLoop pumps messages from the output channel to the websocket connection.
func (c *conn) WriteLoop(out <-chan []byte) {
	//ping ticker
	ticker := time.NewTicker(c.pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()
	for {
		select {
		case message, ok := <-out:
			if !ok {
				if log.GetLevel() >= log.InfoLevel {
					log.WithFields(log.Fields{"logger": "ws.connection", "method": "WriteLoop", "message": message}).
						Info("The out channel is closed; aborting the write loop")
				}
				return
			}
			if err := c.write(websocket.TextMessage, message); err != nil {
				if log.GetLevel() >= log.InfoLevel {
					log.WithFields(log.Fields{"logger": "ws.connection", "method": "WriteLoop", "message": message}).
						WithError(err).Info("Error sending text message into the socket; aborting the write loop")
				}
				return
			}
		case <-ticker.C:
			log.Info("Writing ping")
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				if log.GetLevel() >= log.InfoLevel {
					log.WithFields(log.Fields{"logger": "ws.connection", "method": "WriteLoop"}).
						WithError(err).Info("Error sending ping into the socket; aborting the write loop")
				}
				return
			}
			go c.waitForPong()
		case <-c.control:
			return
		}
	}
}

func (c *conn) waitForPong() {
	log.WithFields(log.Fields{"logger": "ws.connection", "method": "waitForPong"}).
		Info("Setting pong timer")
	select {
	case <-time.After(c.pingTimeout):
		log.WithFields(log.Fields{"logger": "ws.connection", "method": "waitForPong"}).
			Info("Websocket connection timed out")
		c.Close()
	case <-(*c).pong:
		if log.GetLevel() >= log.DebugLevel {
			log.WithFields(log.Fields{"logger": "ws.connection", "method": "waitForPong"}).
				Info("Pong timeout cancelled")
		}
	}
}

func (c *conn) pongHandler(appData string) error {
	c.pong <- true
	return nil
}
