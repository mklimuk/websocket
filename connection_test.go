package websocket

import (
	"errors"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ConnectionTestSuite struct {
	suite.Suite
}

func (suite *ConnectionTestSuite) SetupSuite() {
	log.SetLevel(log.DebugLevel)
}

func (suite *ConnectionTestSuite) TestConstructor() {
	ws := RawConnectionMock{}
	ws.On("SetPongHandler", mock.Anything).Return()
	a := assert.New(suite.T())
	c := newConnection(&ws, "remote", []string{})
	a.NotNil(c.Out)
	a.NotNil(c.In)
	a.NotNil(c.control)
	a.NotEmpty(c.ID)
	a.Equal("remote", c.Remote)
}

func (suite *ConnectionTestSuite) TestClose() {
	ws := RawConnectionMock{}
	ws.On("SetPongHandler", mock.Anything).Return()
	ws.On("SetWriteDeadline", mock.AnythingOfType("time.Time")).Return(nil).Once()
	ws.On("WriteMessage", websocket.CloseMessage, []byte{0x3, 0xed}).Return(nil)
	ws.On("Close").Return(nil)
	c := newConnection(&ws, "localhost", []string{})
	c.Close()
	a := assert.New(suite.T())
	a.Equal(c.state, StateClosing)
	c.OnClose = func(code int, text string, origin DisconnectOrigin) error {
		a.Equal(Self, origin)
		return nil
	}
	c.handleCloseMessage(CloseNoStatusReceived, "")
	a.Equal(c.state, StateClosed)
	c = newConnection(&ws, "localhost", []string{})
	ws.On("SetWriteDeadline", mock.AnythingOfType("time.Time")).Return(errors.New("test error")).Once()
	c.Close()
	a.Equal(c.state, StateClosing)
	c.Close()
	a.Equal(c.state, StateClosed)
	ws.AssertExpectations(suite.T())
}

func (suite *ConnectionTestSuite) TestWriteLoop() {
	ws := RawConnectionMock{}
	ws.On("SetPongHandler", mock.Anything).Return()
	c := newConnection(&ws, "localhost", []string{})
	go c.WriteLoop()
	msg := []byte{'a', 'b', 'c'}
	ws.On("SetWriteDeadline", mock.AnythingOfType("time.Time")).Return(nil).Once()
	ws.On("WriteMessage", websocket.TextMessage, msg).Return(nil).Once()
	c.Out <- Message{MessageType: TextMessage, Payload: msg}
	ws.On("SetWriteDeadline", mock.AnythingOfType("time.Time")).Return(nil).Twice()
	ws.On("WriteMessage", websocket.BinaryMessage, msg).Return(errors.New("test error")).Once()
	res := []byte{0x3, 0xee}
	e := "Error sending text message into the websocket"
	res = append(res, []byte(e)...)
	ws.On("WriteMessage", websocket.CloseMessage, res).Return(nil).Once()
	c.Out <- Message{MessageType: BinaryMessage, Payload: []byte{'a', 'b', 'c'}}
	time.Sleep(100 * time.Millisecond)
}

func (suite *ConnectionTestSuite) TestWritePing() {
	ws := RawConnectionMock{}
	ws.On("SetPongHandler", mock.Anything).Return()
	ws.On("SetWriteDeadline", mock.AnythingOfType("time.Time")).Return(nil)
	ws.On("WriteMessage", websocket.PingMessage, []byte{}).Return(nil)
	ws.On("WriteMessage", websocket.CloseMessage, []byte{0x3, 0xe8}).Return(nil).Once()
	c := newConnection(&ws, "localhost", []string{})
	c.PingPeriod = 500 * time.Millisecond
	c.PingTimeout = 200 * time.Millisecond
	go c.WriteLoop()
	time.Sleep(800 * time.Millisecond)
	//the connection should timeout and close
	ws.AssertExpectations(suite.T())
}

func (suite *ConnectionTestSuite) TestReadLoop() {
	ws := RawConnectionMock{}
	ws.On("SetPongHandler", mock.Anything).Return()
	c := newConnection(&ws, "localhost", []string{})
	msg := []byte{'t', 'e', 's', 't'}
	ws.On("SetReadLimit", int64(32768)).Return()
	ws.On("ReadMessage").Return(websocket.TextMessage, msg, nil).Once()
	ws.On("ReadMessage").Return(websocket.BinaryMessage, msg, nil).Once()
	ws.On("ReadMessage").Return(websocket.BinaryMessage, []byte{}, errors.New("read error")).Once()
	ws.On("SetWriteDeadline", mock.AnythingOfType("time.Time")).Return(nil).Once()
	res := []byte{0x3, 0xee}
	e := "Unexpected websocket error received"
	res = append(res, []byte(e)...)
	ws.On("WriteMessage", websocket.CloseMessage, res).Return(nil).Once()
	go c.ReadLoop()
	a := assert.New(suite.T())
	time.Sleep(100 * time.Millisecond)
	in := <-c.In
	a.Len(in.Payload, 4)
}

func TestConnectionTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectionTestSuite))
}
