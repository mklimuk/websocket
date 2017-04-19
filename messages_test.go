package websocket

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type MessagesTestSuite struct {
	suite.Suite
}

func (suite *MessagesTestSuite) TestEncode() {
	t := &struct {
		ID      string `json:"id"`
		One     int    `json:"one"`
		private string
	}{"test", 1, "noshow"}
	b := EncodeMessage("test:msg", t)
	a := assert.New(suite.T())
	a.Equal(`{"title":"test:msg","body":"{\"id\":\"test\",\"one\":1}"}`, string(b))
}

func TestMessagesTestSuite(t *testing.T) {
	suite.Run(t, new(MessagesTestSuite))
}
