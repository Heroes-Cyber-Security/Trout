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
	sem    chan struct{}
}

func newListener(ctx context.Context, cancel context.CancelFunc, ln net.Listener, cfg ChallengeConfig, maxConns int) *listener {
	return &listener{
		ctx:    ctx,
		cancel: cancel,
		ln:     ln,
		cfg:    cfg,
		sem:    make(chan struct{}, maxConns),
	}
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
		select {
		case l.sem <- struct{}{}:
			go func() {
				defer func() { <-l.sem }()
				handleSession(conn, l.cfg, m)
			}()
		default:
			conn.Close()
			m.log.Warn("connection limit reached, dropped")
		}
	}
}
