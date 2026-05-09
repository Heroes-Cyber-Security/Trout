package netcat

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"hcs.ctf/trout/internal/config"
)

type TokenVerifier func(token string) (userID int, err error)

type FlagGenerator func(baseFlag string, userID int, challengeID string) string

type Logger func(eventType, source string, payload interface{})

type DiscordNotifier func(eventType string, fields map[string]string)

type ChallengeConfig struct {
	ID        string
	Name      string
	Questions []QnA
	BaseFlag  string
	LeetRules string
}

type Manager struct {
	mu         sync.Mutex
	listeners  map[string]*listener
	store      *config.Store
	verify     TokenVerifier
	genFlag    FlagGenerator
	logEvent   Logger
	notify     DiscordNotifier
	log        *slog.Logger
}

func NewManager(store *config.Store, verify TokenVerifier, genFlag FlagGenerator, logEvent Logger, notify DiscordNotifier) *Manager {
	return &Manager{
		listeners: make(map[string]*listener),
		store:     store,
		verify:    verify,
		genFlag:   genFlag,
		logEvent:  logEvent,
		notify:    notify,
		log:       slog.With("component", "netcat"),
	}
}

func (m *Manager) Start(challenge config.Challenge) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.listeners[challenge.ID]; ok {
		return fmt.Errorf("listener already running for %s", challenge.ID)
	}

	questions, err := parseQuestions(challenge.Questions)
	if err != nil {
		return fmt.Errorf("parse questions: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", challenge.NetcatPort))
	if err != nil {
		cancel()
		return fmt.Errorf("listen port %d: %w", challenge.NetcatPort, err)
	}

	l := &listener{
		ctx:    ctx,
		cancel: cancel,
		ln:     ln,
		cfg: ChallengeConfig{
			ID:        challenge.ID,
			Name:      challenge.Name,
			Questions: questions,
			BaseFlag:  challenge.BaseFlag,
			LeetRules: challenge.LeetRules,
		},
	}

	m.listeners[challenge.ID] = l
	go l.serve(m)

	m.log.Info("netcat listener started", "challenge", challenge.ID, "port", challenge.NetcatPort)
	return nil
}

func (m *Manager) Stop(challengeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	l, ok := m.listeners[challengeID]
	if !ok {
		return fmt.Errorf("no listener for %s", challengeID)
	}

	l.cancel()
	l.ln.Close()
	delete(m.listeners, challengeID)
	m.log.Info("netcat listener stopped", "challenge", challengeID)
	return nil
}

func (m *Manager) IsRunning(challengeID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.listeners[challengeID]
	return ok
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, l := range m.listeners {
		l.cancel()
		l.ln.Close()
		delete(m.listeners, id)
	}
}
