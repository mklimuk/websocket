package websocket

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type HubTestSuite struct {
	suite.Suite
}

func (suite *HubTestSuite) SetupSuite() {
}

func (suite *HubTestSuite) TestConstructor() {
	h := NewHub()
	under := h.(*hub)
	a := assert.New(suite.T())
	a.NotNil(under.channels)
	a.NotNil(under.listeners)
}

func (suite *HubTestSuite) TestRegister() {
	h := NewHub()
	under := h.(*hub)
	f := FactoryMock{}
	ws := RawConnectionMock{}
	ws.On("SetPongHandler", mock.Anything).Return()
	ws.On("SetReadLimit", mock.Anything).Return()
	ws.On("ReadMessage").After(3 * time.Second).Return([]byte{})
	c1 := newConnection(&ws, "localhost", []string{})
	c1.ID = "conn1"
	c2 := newConnection(&ws, "localhost", []string{})
	c2.ID = "conn2"
	f.On("UpgradeConnection", mock.Anything, mock.Anything, mock.Anything).Return(c1, nil).Once()
	f.On("UpgradeConnection", mock.Anything, mock.Anything, mock.Anything).Return(c2, nil).Once()
	under.factory = &f
	h.RegisterConnection(nil, &http.Request{}, []string{"test1", "test2"})
	a := assert.New(suite.T())
	time.Sleep(50 * time.Millisecond)
	a.Len(under.channels, 3)
	a.Len(under.channels["test1"], 1)
	a.Len(under.channels["test2"], 1)
	a.Len(under.channels["general"], 1)
	h.RegisterConnection(nil, &http.Request{}, []string{"test1"})
	time.Sleep(50 * time.Millisecond)
	a.Len(under.channels, 3)
	a.Len(under.channels["test1"], 2)
	a.Len(under.channels["test2"], 1)
	a.Len(under.channels["general"], 2)
}

func (suite *HubTestSuite) TestListeners() {
	raw := &RawConnectionMock{}
	raw.On("SetPongHandler", mock.Anything).Return()
	c1 := newConnection(raw, "remote1", []string{"test1"})
	ch := map[string]map[string]*Connection{
		"test1": map[string]*Connection{
			"conn11": c1,
		},
	}
	ch["test1"]["conn11"].ID = "conn11"
	h := NewHub()
	under := h.(*hub)
	under.channels = ch
	go under.listen(ch["test1"]["conn11"])
	// testing listeners
	l := ListenerMock{}
	msg := []byte{'t', 'e', 's', 't'}
	l.On("Handle", msg).Return().Once()
	h.RegisterListener("test1", &l)
	c1.In <- msg
	time.Sleep(50 * time.Millisecond)
	c1.control <- true
	time.Sleep(50 * time.Millisecond)
	l.AssertExpectations(suite.T())
}

func (suite *HubTestSuite) TestBroadcast() {
	raw1 := &RawConnectionMock{}
	raw1.On("SetPongHandler", mock.Anything).Return()
	c1 := newConnection(raw1, "remote1", []string{"test1"})
	c1.ID = "conn11"
	raw2 := &RawConnectionMock{}
	raw2.On("SetPongHandler", mock.Anything).Return()
	c2 := newConnection(raw2, "remote2", []string{"test1"})
	c2.ID = "conn12"
	raw3 := &RawConnectionMock{}
	raw3.On("SetPongHandler", mock.Anything).Return()
	c3 := newConnection(raw3, "remote3", []string{"test2"})
	c3.ID = "conn21"
	ch := map[string]map[string]*Connection{
		"test1": map[string]*Connection{
			"conn11": c1,
			"conn12": c2,
		},
		"test2": map[string]*Connection{
			"conn21": c3,
		},
	}
	h := NewHub()
	under := h.(*hub)
	under.channels = ch
	msg := []byte{'t', 'e', 's', 't'}
	h.Broadcast(Message{MessageType: TextMessage, Payload: msg}, "test1")
	time.Sleep(50 * time.Millisecond)
	a := assert.New(suite.T())
	a.Equal("test", string(Message(<-c1.Out).Payload))
	a.Equal("test", string(Message(<-c2.Out).Payload))
}

func TestHubTestSuite(t *testing.T) {
	suite.Run(t, new(HubTestSuite))
}
