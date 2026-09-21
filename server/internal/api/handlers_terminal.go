package api

import (
	"encoding/binary"
	"net/http"

	"github.com/gorilla/websocket"
)

// handleContainerTerminal upgrades to a WebSocket and relays an interactive
// shell session inside the container.
//
// Client->server messages are framed with a one-byte type so terminal input
// and resize events can share one connection:
//   - 0x00 <bytes...>       — write <bytes...> to the shell's stdin
//   - 0x01 <u16 cols><u16 rows> (big-endian) — resize the pty
//
// Server->client messages are unframed: raw terminal output, written
// straight to the client's terminal renderer (xterm.js).
func (s *Server) handleContainerTerminal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	execID, hijacked, err := s.docker.Exec(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	defer hijacked.Close()

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	done := make(chan struct{})

	// exec output -> websocket
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := hijacked.Reader.Read(buf)
			if n > 0 {
				if werr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// websocket -> exec input / resize
readLoop:
	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil || msgType == websocket.CloseMessage {
			break
		}
		if len(data) == 0 {
			continue
		}
		switch data[0] {
		case 0x00:
			if _, err := hijacked.Conn.Write(data[1:]); err != nil {
				break readLoop
			}
		case 0x01:
			if len(data) >= 5 {
				cols := binary.BigEndian.Uint16(data[1:3])
				rows := binary.BigEndian.Uint16(data[3:5])
				_ = s.docker.ExecResize(r.Context(), execID, uint(rows), uint(cols))
			}
		}
	}

	<-done
}
