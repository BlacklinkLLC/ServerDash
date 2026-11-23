package api

import (
	"encoding/binary"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

func (s *Server) handleHostTerminal(w http.ResponseWriter, r *http.Request) {
	cmd, err := s.hostShellCommand()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	ptmx, tty, err := pty.Open()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: 24, Cols: 80})

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		_ = tty.Close()
		_ = ptmx.Close()
		return
	}
	defer conn.Close()

	if err := cmd.Start(); err != nil {
		_ = tty.Close()
		_ = ptmx.Close()
		_ = conn.WriteMessage(websocket.TextMessage, []byte("error: "+err.Error()))
		return
	}
	_ = tty.Close()

	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
			_ = ptmx.Close()
		})
	}
	defer cleanup()

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, readErr := ptmx.Read(buf)
			if n > 0 {
				if writeErr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); writeErr != nil {
					return
				}
			}
			if readErr != nil {
				return
			}
		}
	}()

readLoop:
	for {
		msgType, data, readErr := conn.ReadMessage()
		if readErr != nil || msgType == websocket.CloseMessage {
			break
		}
		if len(data) == 0 {
			continue
		}
		switch data[0] {
		case 0x00:
			if _, err := ptmx.Write(data[1:]); err != nil {
				break readLoop
			}
		case 0x01:
			if len(data) >= 5 {
				cols := binary.BigEndian.Uint16(data[1:3])
				rows := binary.BigEndian.Uint16(data[3:5])
				_ = pty.Setsize(ptmx, &pty.Winsize{Rows: rows, Cols: cols})
			}
		}
	}

	cleanup()
	deadline := time.Now().Add(500 * time.Millisecond)
	_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), deadline)
	<-done
}

func (s *Server) hostShellCommand() (*exec.Cmd, error) {
	root := s.cfg.HostRoot
	if root == "" {
		for _, shell := range []string{os.Getenv("SHELL"), "/bin/bash", "/bin/sh"} {
			if shell == "" {
				continue
			}
			if st, err := os.Stat(shell); err == nil && st.Mode()&0111 != 0 {
				return exec.Command(shell, "-l"), nil
			}
		}
		return nil, errors.New("no login shell found on this host")
	}

	for _, shell := range []string{"/bin/bash", "/bin/sh"} {
		inside := filepath.Join(root, filepath.FromSlash(shell))
		if st, err := os.Stat(inside); err == nil && st.Mode()&0111 != 0 {
			return exec.Command("chroot", root, shell, "-l"), nil
		}
	}
	return nil, errors.New("no host shell found under SERVERDASH_HOST_ROOT")
}
