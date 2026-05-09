package netcat

import (
	"context"
	"net"
)

type listener struct {
	ctx    context.Context
	cancel context.CancelFunc
	ln     net.Listener
	cfg    ChallengeConfig
}

func (l *listener) serve(m *Manager) {
	for {
		conn, err := l.ln.Accept()
		if err != nil {
			select {
			case <-l.ctx.Done():
				return
			default:
				m.log.Error("accept error", "error", err)
				continue
			}
		}
		go handleSession(conn, l.cfg, m)
	}
}
