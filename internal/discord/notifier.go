package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"hcs.ctf/trout/internal/config"
)

type embedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type embed struct {
	Title       string       `json:"title"`
	Color       int          `json:"color"`
	Fields      []embedField `json:"fields"`
	Timestamp   string       `json:"timestamp"`
}

type webhookPayload struct {
	Embeds []embed `json:"embeds"`
}

var eventColors = map[string]int{
	"flag_generated":  5814783,
	"solve":           15548997,
	"first_blood":     15844367,
	"challenge_created": 3447003,
}

var eventTitles = map[string]string{
	"flag_generated":  "Flag Generated",
	"solve":           "Challenge Solved",
	"first_blood":     "First Blood",
	"challenge_created": "Challenge Created",
}

type Notifier struct {
	store *config.Store
	client *http.Client
	log    *slog.Logger
}

func New(store *config.Store) *Notifier {
	return &Notifier{
		store:  store,
		client: &http.Client{Timeout: 10 * time.Second},
		log:    slog.With("component", "discord"),
	}
}

func (n *Notifier) Send(eventType string, fields map[string]string) {
	if n == nil {
		return
	}

	webhooks, err := n.store.GetDiscordWebhooksByEvent(eventType)
	if err != nil || len(webhooks) == 0 {
		return
	}

	title, ok := eventTitles[eventType]
	if !ok {
		title = eventType
	}

	color, ok := eventColors[eventType]
	if !ok {
		color = 7506394
	}

	var embedFields []embedField
	for k, v := range fields {
		embedFields = append(embedFields, embedField{
			Name:   k,
			Value:  v,
			Inline: true,
		})
	}

	payload := webhookPayload{
		Embeds: []embed{{
			Title:     title,
			Color:     color,
			Fields:    embedFields,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		n.log.Error("marshal payload", "error", err)
		return
	}

	for _, wh := range webhooks {
		if err := n.send(wh.URL, body); err != nil {
			n.log.Error("send webhook", "url", wh.URL, "error", err)
		}
	}
}

func (n *Notifier) send(url string, body []byte) error {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}
