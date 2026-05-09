package netcat

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"
)

type QnA struct {
	Question string `json:"q"`
	Answer   string `json:"a"`
}

func parseQuestions(raw string) ([]QnA, error) {
	if raw == "" {
		return nil, nil
	}
	var parsed []QnA
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("parse questions json: %w", err)
	}
	return parsed, nil
}

func handleSession(conn net.Conn, cfg ChallengeConfig, m *Manager) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Minute))

	wr := func(format string, args ...interface{}) {
		fmt.Fprintf(conn, format+"\r\n", args...)
	}

	wr("=== %s ===", cfg.Name)
	wr("")

	rd := bufio.NewReader(conn)
	for i, q := range cfg.Questions {
		wr("Q%d: %s", i+1, q.Question)
		wr("> ")

		answer, err := rd.ReadString('\n')
		if err != nil {
			wr("Connection error.")
			return
		}
		answer = strings.TrimSpace(answer)

		if !strings.EqualFold(answer, q.Answer) {
			wr("Wrong. Disconnecting.")
			return
		}
		wr("Correct!")
		wr("")
	}

	wr("Enter your CTFd access token (or type 'skip' for anonymous):")
	wr("> ")
	token, err := rd.ReadString('\n')
	if err != nil {
		wr("Connection error.")
		return
	}
	token = strings.TrimSpace(token)

	var userID int
	if strings.ToLower(token) == "skip" || token == "" {
		n, err := rand.Int(rand.Reader, big.NewInt(1<<31))
		if err != nil {
			wr("Internal error.")
			return
		}
		userID = int(n.Int64())
	} else {
		id, err := m.verify(token)
		if err != nil {
			wr("Invalid token. Disconnecting.")
			return
		}
		userID = id
	}

	generated := m.genFlag(cfg.BaseFlag, userID, cfg.ID)
	wr("Your flag: %s", generated)

	fields := map[string]string{
		"challenge": cfg.ID,
		"user_id":   fmt.Sprintf("%d", userID),
		"flag":      generated,
	}
	m.logEvent("flag_generated", "netcat", fields)
	m.notify("flag_generated", fields)

	time.Sleep(2 * time.Second)
}
