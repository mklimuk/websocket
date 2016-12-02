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
	h := NewHub(nil)
	under := h.(*hub)
	assert.NotNil(suite.T(), under.channels)
	assert.NotNil(suite.T(), under.listeners)
}

func (suite *HubTestSuite) TestRegister() {
	h := NewHub(nil)
	under := h.(*hub)
	f := factoryMock{}
	c := &ConnectionMock{}
	c.On("ID").Return("conn1").Once()
	c.On("ID").Return("conn2").Once()
	c.On("Host").Return("localhost")
	f.On("UpgradeConnection", mock.Anything, mock.Anything, mock.Anything).Return(c, nil)
	under.factory = &f
	h.RegisterConnection(nil, &http.Request{}, []string{"test1", "test2"}, none)
	time.Sleep(50 * time.Millisecond)
	assert.Len(suite.T(), under.channels, 3)
	assert.Len(suite.T(), under.channels["test1"], 1)
	assert.Len(suite.T(), under.channels["test2"], 1)
	assert.Len(suite.T(), under.channels["general"], 1)
	h.RegisterConnection(nil, &http.Request{}, []string{"test1"}, none)
	time.Sleep(50 * time.Millisecond)
	assert.Len(suite.T(), under.channels, 3)
	assert.Len(suite.T(), under.channels["test1"], 2)
	assert.Len(suite.T(), under.channels["test2"], 1)
	assert.Len(suite.T(), under.channels["general"], 2)
}

func (suite *HubTestSuite) TestListeners() {
	ch := map[string]map[string]Connection{
		"test1": map[string]Connection{
			"conn11": &ConnectionMock{},
		},
	}
	c1 := make(chan []byte, 32)
	c2 := make(chan string, 4)
	c3 := make(chan bool, 1)
	ch["test1"]["conn11"].(*ConnectionMock).On("In").Return(c1, c2).Once()
	ch["test1"]["conn11"].(*ConnectionMock).On("Control").Return(c3).Once()
	ch["test1"]["conn11"].(*ConnectionMock).On("Channels").Return([]string{"test1"})
	ch["test1"]["conn11"].(*ConnectionMock).On("ID").Return("conn11")
	h := NewHub(nil)
	under := h.(*hub)
	under.channels = ch
	go under.readFrom(ch["test1"]["conn11"])
	// testing listeners
	l := ListenerMock{}
	msg := []byte{'t', 'e', 's', 't'}
	l.On("Handle", msg).Return().Once()
	h.RegisterListener("test1", &l)
	c1 <- msg
	time.Sleep(50 * time.Millisecond)
	ch["test1"]["conn11"].(*ConnectionMock).AssertExpectations(suite.T())
	ch["test1"]["conn11"].(*ConnectionMock).On("Out").Return(make(chan []byte)).Once()
	ch["test1"]["conn11"].(*ConnectionMock).On("Close").Return().Once()
	c3 <- true
	time.Sleep(50 * time.Millisecond)
	l.AssertExpectations(suite.T())
}

func (suite *HubTestSuite) TestBroadcast() {
	ch := map[string]map[string]Connection{
		"test1": map[string]Connection{
			"conn11": &ConnectionMock{},
			"conn12": &ConnectionMock{},
		},
		"test2": map[string]Connection{
			"conn21": &ConnectionMock{},
		},
	}
	h := NewHub(nil)
	under := h.(*hub)
	under.channels = ch
	c1 := make(chan []byte, 2)
	ch["test1"]["conn11"].(*ConnectionMock).On("Out").Return(c1)
	ch["test1"]["conn11"].(*ConnectionMock).On("ID").Return("conn1")
	ch["test1"]["conn11"].(*ConnectionMock).On("Host").Return("localhost")
	ch["test1"]["conn12"].(*ConnectionMock).On("Out").Return(c1)
	ch["test1"]["conn12"].(*ConnectionMock).On("ID").Return("conn1")
	ch["test1"]["conn12"].(*ConnectionMock).On("Host").Return("localhost")
	msg := []byte{'t', 'e', 's', 't'}
	h.Broadcast(msg, "test1")
	time.Sleep(50 * time.Millisecond)
	assert.Equal(suite.T(), "test", string(<-c1))
	assert.Equal(suite.T(), "test", string(<-c1))
	ch["test1"]["conn11"].(*ConnectionMock).AssertExpectations(suite.T())
	ch["test1"]["conn12"].(*ConnectionMock).AssertExpectations(suite.T())
}

func TestHubTestSuite(t *testing.T) {
	suite.Run(t, new(HubTestSuite))
}
