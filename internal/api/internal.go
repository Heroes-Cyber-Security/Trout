package api

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strconv"
)

type FlagGenerator func(baseFlag string, userID int, challengeID string) string

type ChallengeLookup func(id string) (baseFlag string, ok bool)

type InternalHandler struct {
	genFlag  FlagGenerator
	lookup   ChallengeLookup
	apiKey   string
	log      *slog.Logger
}

func NewInternal(genFlag FlagGenerator, lookup ChallengeLookup, apiKey string) *InternalHandler {
	return &InternalHandler{
		genFlag: genFlag,
		lookup:  lookup,
		apiKey:  apiKey,
		log:     slog.With("component", "internal_api"),
	}
}

func (h *InternalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		http.Error(w, "bad remote addr", http.StatusBadRequest)
		return
	}

	ip := net.ParseIP(host)
	if ip == nil {
		http.Error(w, "bad ip", http.StatusBadRequest)
		return
	}

	if !ip.IsLoopback() && !ip.IsPrivate() {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if r.Header.Get("X-Internal-Api-Key") != h.apiKey {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	userIDStr := r.URL.Query().Get("user_id")
	challengeID := r.URL.Query().Get("challenge_id")

	if userIDStr == "" || challengeID == "" {
		http.Error(w, "missing user_id or challenge_id", http.StatusBadRequest)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "invalid user_id", http.StatusBadRequest)
		return
	}

	baseFlag, ok := h.lookup(challengeID)
	if !ok {
		http.Error(w, "challenge not found", http.StatusNotFound)
		return
	}

	flag := h.genFlag(baseFlag, userID, challengeID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"flag":         flag,
		"user_id":      userID,
		"challenge_id": challengeID,
	})
}
