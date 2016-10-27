package websocket

/*
func handlers(h *hub, b *event.Bus) {
	go h.clock()
	b.Subscribe(event.QueueContent, (*h).message)
	b.Subscribe(event.PlaybackStart, (*h).message)
	b.Subscribe(event.PlaybackEnd, (*h).message)
}

func (h *hub) message(title event.Type, m string) error {
	var msg []byte
	var err error
	if msg, err = JSONMessage(string(title), m, ""); err != nil {
		log.WithFields(log.Fields{"logger": "ws.handlers", "method": "timetable"}).
			WithError(err).Error("Could not send request.")
		return err
	}
	if log.GetLevel() >= log.DebugLevel {
		log.WithFields(log.Fields{"logger": "ws.handlers", "method": "message", "message": string(msg)}).
			Debug("Broadcasting current timetable.")
	}
	h.Broadcast() <- msg
	return nil
}

//clock handles realtime clock on clients
func (h *hub) clock() {
	ticker := time.NewTicker(time.Second * 1)
	for {
		<-ticker.C
		params := make(map[string]string)
		params["time"] = time.Now().Format(time.RFC3339Nano)
		var req []byte
		var err error
		if req, err = json.Marshal(params); err != nil {
			log.WithFields(log.Fields{"logger": "ws.handlers", "method": "clock"}).
				WithError(err).Error("Could not marshal clock request.")
		}
		msg, err := JSONMessage("SET_TIME", string(req), "")
		if err != nil {
			log.WithFields(log.Fields{"logger": "ws.handlers", "method": "clock"}).
				WithError(err).Error("Could not send clock request.")
		}
		(*h).Broadcast() <- msg
	}
}
*/
