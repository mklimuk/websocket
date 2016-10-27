package websocket

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type MessagesTestSuite struct {
	suite.Suite
}

func (suite *MessagesTestSuite) TestConstructor() {
	m, err := JSONMessage("title", "body", "")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), `{"title":"title","body":"body"}`, string(m))
}

func TestMessagesTestSuite(t *testing.T) {
	suite.Run(t, new(MessagesTestSuite))
}
