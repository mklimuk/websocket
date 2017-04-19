package websocket

import (
	"testing"

	"github.com/stretchr/testify/mock"
)

func TestMocks(t *testing.T) {
	var h Hub
	h = &HubMock{}
	h.(*HubMock).On("Broadcast", mock.Anything, mock.Anything).Return()
}
