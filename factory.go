package websocket

import (
	"net/http"

	ws "github.com/gorilla/websocket"
)

var upgrader = ws.Upgrader{
	ReadBufferSize:  2048,
	WriteBufferSize: 2048,
}

//ConnectionFactory is a wrapper interface over gorilla websocket upgrader struct
type ConnectionFactory interface {
	UpgradeConnection(writer http.ResponseWriter, req *http.Request, channels []string) (Connection, error)
}

type gorillaFactory struct {
	u ws.Upgrader
}

//NewFactory is the connection factory constructor
func NewFactory() ConnectionFactory {
	g := gorillaFactory{ws.Upgrader{}}
	return &g
}

//UpgradeConnection upgrades HTTP connection to a websocket
func (g *gorillaFactory) UpgradeConnection(writer http.ResponseWriter, req *http.Request, channels []string) (Connection, error) {
	var err error
	var w *ws.Conn
	if w, err = g.u.Upgrade(writer, req, nil); err != nil {
		return nil, err
	}
	return newConnection(w, make(chan []byte, 32), req.Host, channels), nil
}
