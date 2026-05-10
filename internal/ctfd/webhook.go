package ctfd

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

type SubmissionPayload struct {
	ID          int         `json:"id"`
	ChallengeID int         `json:"challenge_id"`
	User        interface{} `json:"user"`
	Team        interface{} `json:"team"`
	Type        string      `json:"type"`
	Date        string      `json:"date"`
	Challenge   interface{} `json:"challenge"`
}

type WebhookHandler struct {
	secret string
	notify func(eventType string, fields map[string]string)
	log    *slog.Logger
}

func NewWebhookHandler(secret string, notify func(eventType string, fields map[string]string)) *WebhookHandler {
	return &WebhookHandler{
		secret: secret,
		notify: notify,
		log:    slog.With("component", "ctfd_webhook"),
	}
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		h.handleEvent(w, r)
		return
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (h *WebhookHandler) VerifyHandler(w http.ResponseWriter, r *http.Request) {
	h.handleVerify(w, r)
}

func (h *WebhookHandler) handleVerify(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}

	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write([]byte("verify:" + token))
	sig := hex.EncodeToString(mac.Sum(nil))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"response": sig})
}

func (h *WebhookHandler) handleEvent(w http.ResponseWriter, r *http.Request) {
	sigHeader := r.Header.Get("CTFd-Webhook-Signature")
	eventType := r.Header.Get("CTFd-Webhook-Event")

	if sigHeader == "" {
		http.Error(w, "missing signature", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	if h.secret == "" {
		h.log.Error("webhook secret is not configured, rejecting event")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if err := verifySignature(sigHeader, body, h.secret); err != nil {
		h.log.Warn("invalid signature", "error", err)
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var payload SubmissionPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.log.Warn("parse payload", "error", err)
		http.Error(w, "parse payload", http.StatusBadRequest)
		return
	}

	fields := map[string]string{
		"event":     eventType,
		"challenge": fmt.Sprintf("%d", payload.ChallengeID),
		"type":      payload.Type,
	}

	if payload.User != nil {
		if u, ok := payload.User.(map[string]interface{}); ok {
			if id, ok := u["id"]; ok {
				fields["user_id"] = fmt.Sprintf("%v", id)
			}
			if name, ok := u["name"]; ok {
				fields["user_name"] = fmt.Sprintf("%v", name)
			}
		}
	}

	if h.notify != nil {
		h.notify(eventType, fields)
	}

	w.WriteHeader(http.StatusOK)
}

func verifySignature(header string, body []byte, secret string) error {
	parts := strings.SplitN(header, ",", 2)
	if len(parts) != 2 {
		return fmt.Errorf("malformed header")
	}

	tPart := strings.TrimPrefix(parts[0], "t=")
	v1Part := strings.TrimPrefix(parts[1], "v1=")

	_, err := strconv.ParseInt(tPart, 10, 64)
	if err != nil {
		return fmt.Errorf("bad timestamp: %w", err)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(tPart + "." + string(body)))
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(v1Part)) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}
