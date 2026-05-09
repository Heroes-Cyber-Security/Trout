package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

type SubmissionPayload struct {
	UserID      string `json:"user_id"`
	UserName    string `json:"user_name"`
	ChallengeID string `json:"challenge_id"`
	Challenge   string `json:"challenge_name"`
	Flag        string `json:"flag"`
	Status      string `json:"status"`
}

type SubmissionsHandler struct {
	secret     string
	onSubmit   func(eventType string, fields map[string]string)
	log        *slog.Logger
}

func NewSubmissions(secret string, onSubmit func(eventType string, fields map[string]string)) *SubmissionsHandler {
	return &SubmissionsHandler{
		secret:   secret,
		onSubmit: onSubmit,
		log:      slog.With("component", "submissions_api"),
	}
}

func (h *SubmissionsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	if h.secret != "" {
		sig := r.Header.Get("X-Trout-Signature")
		if sig == "" {
			http.Error(w, "missing signature", http.StatusUnauthorized)
			return
		}
		if !verifyHMAC(body, sig, h.secret) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	var payload SubmissionPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	fields := map[string]string{
		"user_id":      payload.UserID,
		"user_name":    payload.UserName,
		"challenge_id": payload.ChallengeID,
		"challenge":    payload.Challenge,
		"flag":         payload.Flag,
		"status":       payload.Status,
	}

	if h.onSubmit != nil {
		h.onSubmit("submission", fields)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func verifyHMAC(body []byte, sig, secret string) bool {
	parts := strings.SplitN(sig, ",", 2)
	if len(parts) != 2 {
		return false
	}
	ts := strings.TrimPrefix(parts[0], "t=")
	v1 := strings.TrimPrefix(parts[1], "v1=")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "." + string(body)))
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(v1))
}
