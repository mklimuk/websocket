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
	c := newConnection(&ws, make(chan []byte, 1), "localhost", []string{})
	assert.NotNil(suite.T(), c.Out())
	in, intxt := c.In()
	assert.NotNil(suite.T(), in)
	assert.NotNil(suite.T(), intxt)
	assert.NotNil(suite.T(), c.Control())
	assert.NotEmpty(suite.T(), c.ID())
	assert.Equal(suite.T(), "localhost", c.Host())
}

func (suite *ConnectionTestSuite) TestClose() {
	ws := RawConnectionMock{}
	ws.On("SetPongHandler", mock.Anything).Return()
	ws.On("Close").Return(nil).Once()
	ws.On("SetWriteDeadline", mock.AnythingOfType("time.Time")).Return(nil).Once()
	ws.On("WriteMessage", websocket.CloseMessage, []byte{}).Return(nil)
	c := newConnection(&ws, make(chan []byte, 1), "localhost", []string{})
	c.Close()
	c = newConnection(&ws, make(chan []byte, 1), "localhost", []string{})
	ws.On("Close").Return(nil).Once()
	ws.On("SetWriteDeadline", mock.AnythingOfType("time.Time")).Return(errors.New("test error")).Once()
	c.Close()
	c = newConnection(&ws, make(chan []byte, 1), "localhost", []string{})
	ws.On("SetWriteDeadline", mock.AnythingOfType("time.Time")).Return(nil).Once()
	ws.On("Close").Return(errors.New("test error")).Once()
	c.Close()
	ws.AssertExpectations(suite.T())
}

func (suite *ConnectionTestSuite) TestWriteLoop() {
	ws := RawConnectionMock{}
	ws.On("SetPongHandler", mock.Anything).Return()
	c := newConnection(&ws, make(chan []byte, 1), "localhost", []string{})
	go c.WriteLoop(c.Out())
	msg := []byte{'a', 'b', 'c'}
	ws.On("SetWriteDeadline", mock.AnythingOfType("time.Time")).Return(nil).Once()
	ws.On("WriteMessage", websocket.TextMessage, msg).Return(nil).Once()
	c.Out() <- []byte{'a', 'b', 'c'}
	ws.On("SetWriteDeadline", mock.AnythingOfType("time.Time")).Return(nil).Twice()
	ws.On("WriteMessage", websocket.TextMessage, msg).Return(errors.New("test error")).Once()
	ws.On("WriteMessage", websocket.CloseMessage, []byte{}).Return(nil).Once()
	ws.On("Close").Return(nil).Once()
	c.Out() <- []byte{'a', 'b', 'c'}
	time.Sleep(100 * time.Millisecond)
	ws.AssertExpectations(suite.T())
}

func (suite *ConnectionTestSuite) TestWritePing() {
	ws := RawConnectionMock{}
	ws.On("SetPongHandler", mock.Anything).Return()
	ws.On("SetWriteDeadline", mock.AnythingOfType("time.Time")).Return(nil)
	ws.On("WriteMessage", websocket.PingMessage, []byte{}).Return(nil).Once()
	ws.On("WriteMessage", websocket.CloseMessage, []byte{}).Return(nil).Once()
	ws.On("Close").Return(nil).Once()
	c := newConnection(&ws, make(chan []byte, 1), "localhost", []string{})
	c.(*conn).pingPeriod = 500 * time.Millisecond
	c.(*conn).pingTimeout = 200 * time.Millisecond
	go c.WriteLoop(c.Out())
	time.Sleep(800 * time.Millisecond)
	//the connection should timeout and close
	ws.AssertExpectations(suite.T())
}

func (suite *ConnectionTestSuite) TestReadLoop() {
	ws := RawConnectionMock{}
	ws.On("SetPongHandler", mock.Anything).Return()
	c := newConnection(&ws, make(chan []byte, 1), "localhost", []string{})
	bin, txt := c.In()
	msg := []byte{'t', 'e', 's', 't'}
	ws.On("SetReadLimit", int64(2048)).Return()
	ws.On("ReadMessage").Return(websocket.TextMessage, msg, nil).Once()
	ws.On("ReadMessage").Return(websocket.BinaryMessage, msg, nil).Once()
	ws.On("ReadMessage").Return(websocket.BinaryMessage, []byte{}, errors.New("read error")).Once()
	ws.On("SetWriteDeadline", mock.AnythingOfType("time.Time")).Return(nil).Once()
	ws.On("WriteMessage", websocket.CloseMessage, []byte{}).Return(nil).Once()
	ws.On("Close").Return(nil).Once()
	go c.ReadLoop()
	time.Sleep(100 * time.Millisecond)
	assert.Equal(suite.T(), "test", <-txt)
	time.Sleep(100 * time.Millisecond)
	in := <-bin
	assert.Len(suite.T(), in, 4)
	ws.AssertExpectations(suite.T())
}

func TestConnectionTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectionTestSuite))
}
