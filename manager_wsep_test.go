package jasper

import (
	"context"
	"net/http"
	"time"

	"cdr.dev/wsep"
	"github.com/deciduosity/grip"
	"github.com/deciduosity/grip/message"
	"nhooyr.io/websocket"
)

type wsepService struct{}

func (ws *wsepService) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	startAt := time.Now()
	defer grip.Info(func() message.Fields {
		return message.Fields{"op": "ws handled", "outomce": "completed", "dur": time.Since(startAt).String()}
	})
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	conn, err := websocket.Accept(rw, r, nil)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	defer conn.Close(websocket.StatusNormalClosure, "normal closure")

	if err := wsep.Serve(ctx, conn, wsep.LocalExecer{}); err != nil {
		msg := message.WrapError(err, "failed to serve wsep process")
		grip.Error(msg)
		conn.Close(websocket.StatusAbnormalClosure, msg.String())
		return
	}
}
