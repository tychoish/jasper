package jasper

import (
	"net/http"

	"cdr.dev/wsep"
	"github.com/deciduosity/grip"
	"github.com/deciduosity/grip/message"
	"nhooyr.io/websocket"
)

type wsepService struct{}

func (ws *wsepService) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(rw, r, nil)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := wsep.Serve(r.Context(), conn, wsep.LocalExecer{}); err != nil {
		grip.Error(message.WrapError(err, "failed to serve wsep process"))
		conn.Close(websocket.StatusAbnormalClosure, "failed to serve execer")
		return
	}

	conn.Close(websocket.StatusNormalClosure, "normal closure")
}
