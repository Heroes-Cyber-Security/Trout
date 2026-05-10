package ui

import (
	"crypto/subtle"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"hcs.ctf/trout/internal/config"
	"hcs.ctf/trout/internal/ctfd"
)

type Question struct {
	Q string `json:"q"`
	A string `json:"a"`
}

type AdminHandler struct {
	store       *config.Store
	password    string
	ncManager   interface {
		Start(config.Challenge) error
		Stop(string) error
		IsRunning(string) bool
	}
	log         *slog.Logger
	templates   *template.Template
}

func NewAdmin(store *config.Store, password string, ncManager interface {
	Start(config.Challenge) error
	Stop(string) error
	IsRunning(string) bool
}) *AdminHandler {
	return &AdminHandler{
		store:     store,
		password:  password,
		ncManager: ncManager,
		log:       slog.With("component", "admin_ui"),
		templates: parsedTemplates,
	}
}

func (h *AdminHandler) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
			if r.Header.Get("Origin") == "" && r.Header.Get("Referer") == "" {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || subtle.ConstantTimeCompare([]byte(pass), []byte(h.password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Trout Admin"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (h *AdminHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		pass := r.FormValue("password")
		if subtle.ConstantTimeCompare([]byte(pass), []byte(h.password)) == 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Trout Admin"`)
			w.Header().Set("Set-Cookie", "admin_session=1; path=/admin/")
			http.Redirect(w, r, "/admin/", http.StatusSeeOther)
			return
		}
		h.render(w, r, "login.html", map[string]interface{}{
			"Flash":     "Invalid password",
			"FlashType": "error",
		})
		return
	}
	h.render(w, r, "login.html", nil)
}

func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	chals, _ := h.store.ListChallenges()
	events, _ := h.store.ListEvents(10)
	ctfdCfg, _ := h.store.GetCTFdConfig()

	activeCount := 0
	for _, c := range chals {
		if c.Enabled && h.ncManager.IsRunning(c.ID) {
			activeCount++
		}
	}

	type eventRow struct {
		ID      int
		Time    string
		Type    string
		Source  string
		Payload string
	}
	var evs []eventRow
	for _, e := range events {
		evs = append(evs, eventRow{
			ID: e.ID, Time: e.CreatedAt.Format("15:04:05"),
			Type: e.EventType, Source: e.Source, Payload: truncate(e.Payload, 60),
		})
	}

	h.render(w, r, "dashboard.html", map[string]interface{}{
		"ChallengeCount":  len(chals),
		"ActiveListeners": activeCount,
		"CTFdStatus":      ctfdCfg.DetectedEdition,
		"Events":          evs,
	})
}

func (h *AdminHandler) ListChallenges(w http.ResponseWriter, r *http.Request) {
	chals, _ := h.store.ListChallenges()

	type chalRow struct {
		ID            string
		Name          string
		NetcatPort    int
		QuestionCount int
		Enabled       bool
	}
	var rows []chalRow
	for _, c := range chals {
		var qs []Question
		json.Unmarshal([]byte(c.Questions), &qs)
		rows = append(rows, chalRow{
			ID: c.ID, Name: c.Name, NetcatPort: c.NetcatPort,
			QuestionCount: len(qs), Enabled: c.Enabled && h.ncManager.IsRunning(c.ID),
		})
	}

	h.render(w, r, "challenges.html", map[string]interface{}{
		"Challenges": rows,
	})
}

func (h *AdminHandler) NewChallengeForm(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, "challenge_form.html", nil)
}

func (h *AdminHandler) CreateChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	c := h.parseChallengeForm(r)
	if err := h.store.UpsertChallenge(c); err != nil {
		h.renderError(w, r, "Save failed: "+err.Error())
		return
	}

	if c.Enabled {
		if err := h.ncManager.Start(c); err != nil {
			h.store.LogEvent("challenge_error", "admin", map[string]string{
				"challenge": c.ID, "error": err.Error(),
			})
		}
	}

	http.Redirect(w, r, "/admin/challenges", http.StatusSeeOther)
}

func (h *AdminHandler) ViewChallenge(w http.ResponseWriter, r *http.Request) {
	id := extractPathSegment(r.URL.Path, "/admin/challenges/")
	chal, _ := h.store.GetChallenge(id)
	if chal == nil {
		http.NotFound(w, r)
		return
	}

	var qs []Question
	json.Unmarshal([]byte(chal.Questions), &qs)

	h.render(w, r, "challenge_detail.html", map[string]interface{}{
		"Challenge": chal,
		"Questions": qs,
		"Running":   h.ncManager.IsRunning(id),
		"LeetRules": chal.LeetRules,
	})
}

func (h *AdminHandler) EditChallengeForm(w http.ResponseWriter, r *http.Request) {
	id := extractPathSegment(strings.TrimSuffix(r.URL.Path, "/edit"), "/admin/challenges/")
	chal, _ := h.store.GetChallenge(id)
	if chal == nil {
		http.NotFound(w, r)
		return
	}
	h.render(w, r, "challenge_form.html", map[string]interface{}{
		"Challenge": chal,
	})
}

func (h *AdminHandler) UpdateChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := extractPathSegment(strings.TrimSuffix(r.URL.Path, "/edit"), "/admin/challenges/")
	c := h.parseChallengeForm(r)
	c.ID = id

	h.ncManager.Stop(id)

	if err := h.store.UpsertChallenge(c); err != nil {
		h.renderError(w, r, "Save failed: "+err.Error())
		return
	}

	if c.Enabled {
		if err := h.ncManager.Start(c); err != nil {
			h.store.LogEvent("challenge_error", "admin", map[string]string{
				"challenge": c.ID, "error": err.Error(),
			})
		}
	}

	http.Redirect(w, r, "/admin/challenges/"+id, http.StatusSeeOther)
}

func (h *AdminHandler) ToggleChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := extractPathSegment(strings.TrimSuffix(r.URL.Path, "/toggle"), "/admin/challenges/")
	chal, _ := h.store.GetChallenge(id)
	if chal == nil {
		http.NotFound(w, r)
		return
	}

	if h.ncManager.IsRunning(id) {
		h.ncManager.Stop(id)
	} else {
		if err := h.ncManager.Start(*chal); err != nil {
			h.renderError(w, r, "Start failed: "+err.Error())
			return
		}
	}

	http.Redirect(w, r, "/admin/challenges", http.StatusSeeOther)
}

func (h *AdminHandler) DeleteChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := extractPathSegment(strings.TrimSuffix(r.URL.Path, "/delete"), "/admin/challenges/")
	h.ncManager.Stop(id)
	h.store.DeleteChallenge(id)
	http.Redirect(w, r, "/admin/challenges", http.StatusSeeOther)
}

func (h *AdminHandler) CTFdSettings(w http.ResponseWriter, r *http.Request) {
	cfg, _ := h.store.GetCTFdConfig()

	if r.Method == http.MethodPost {
		cfg.URL = strings.TrimRight(r.FormValue("url"), "/")
		cfg.APIKey = r.FormValue("api_key")
		cfg.WebhookSecret = r.FormValue("webhook_secret")
		if v := r.FormValue("poll_interval_sec"); v != "" {
			cfg.PollIntervalSec, _ = strconv.Atoi(v)
		}

		if r.FormValue("action") == "detect" {
			detector := newCTFdDetector(cfg.URL, cfg.APIKey)
			edition, err := detector.Detect()
			if err != nil {
				cfg.DetectedEdition = "error"
			} else {
				cfg.DetectedEdition = edition
			}
		}

		cfg.PluginInstalled = r.FormValue("plugin_installed") == "1"
		h.store.SaveCTFdConfig(*cfg)
		http.Redirect(w, r, "/admin/settings/ctfd", http.StatusSeeOther)
		return
	}

	webhookURL := ""
	if cfg.URL != "" {
		webhookURL = cfg.URL + "/ctfd/webhook"
	}

	h.render(w, r, "settings_ctfd.html", map[string]interface{}{
		"URL":              cfg.URL,
		"APIKey":           cfg.APIKey,
		"WebhookSecret":    cfg.WebhookSecret,
		"PollIntervalSec":  cfg.PollIntervalSec,
		"DetectedEdition":  cfg.DetectedEdition,
		"PluginInstalled":  cfg.PluginInstalled,
		"WebhookURL":       webhookURL,
	})
}

func (h *AdminHandler) DiscordSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		rawURL := r.FormValue("url")
		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			h.renderError(w, r, "Invalid webhook URL: must be https")
			return
		}
		eventType := r.FormValue("event_type")
		h.store.AddDiscordWebhook(config.DiscordWebhook{
			URL: rawURL, EventType: eventType, Enabled: true,
		})
		http.Redirect(w, r, "/admin/settings/discord", http.StatusSeeOther)
		return
	}

	webhooks, _ := h.store.ListDiscordWebhooks()
	type whRow struct {
		ID        int
		URL       string
		EventType string
		Enabled   bool
	}
	var rows []whRow
	for _, w := range webhooks {
		rows = append(rows, whRow{
			ID: w.ID, URL: w.URL, EventType: w.EventType, Enabled: w.Enabled,
		})
	}

	h.render(w, r, "settings_discord.html", map[string]interface{}{
		"Webhooks": rows,
	})
}

func (h *AdminHandler) DeleteDiscordWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	idStr := extractPathSegment(strings.TrimSuffix(r.URL.Path, "/delete"), "/admin/settings/discord/")
	id, _ := strconv.Atoi(idStr)
	h.store.DeleteDiscordWebhook(id)
	http.Redirect(w, r, "/admin/settings/discord", http.StatusSeeOther)
}

func (h *AdminHandler) Logs(w http.ResponseWriter, r *http.Request) {
	events, _ := h.store.ListEvents(100)

	type evRow struct {
		ID      int
		Time    string
		Type    string
		Source  string
		Payload string
	}
	var rows []evRow
	for _, e := range events {
		var prettyJSON string
		var raw interface{}
		if json.Unmarshal([]byte(e.Payload), &raw) == nil {
			b, _ := json.MarshalIndent(raw, "", "  ")
			prettyJSON = string(b)
		} else {
			prettyJSON = e.Payload
		}
		rows = append(rows, evRow{
			ID: e.ID, Time: e.CreatedAt.Format("2006-01-02 15:04:05"),
			Type: e.EventType, Source: e.Source, Payload: prettyJSON,
		})
	}

	h.render(w, r, "logs.html", map[string]interface{}{
		"Events": rows,
	})
}

func (h *AdminHandler) parseChallengeForm(r *http.Request) config.Challenge {
	port, _ := strconv.Atoi(r.FormValue("netcat_port"))
	return config.Challenge{
		ID:          r.FormValue("id"),
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		BaseFlag:    r.FormValue("base_flag"),
		LeetRules:   r.FormValue("leet_rules"),
		Questions:   r.FormValue("questions"),
		NetcatPort:  port,
		Enabled:     r.FormValue("enabled") == "1",
	}
}

func (h *AdminHandler) render(w http.ResponseWriter, r *http.Request, tmpl string, data map[string]interface{}) {
	if data == nil {
		data = map[string]interface{}{}
	}
	flash := r.URL.Query().Get("flash")
	flashType := r.URL.Query().Get("flash_type")
	if flash != "" {
		data["Flash"] = flash
		data["FlashType"] = flashType
		if flashType == "" {
			data["FlashType"] = "success"
		}
	}
	err := h.templates.ExecuteTemplate(w, tmpl, data)
	if err != nil {
		h.log.Error("render template", "error", err)
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func (h *AdminHandler) renderError(w http.ResponseWriter, r *http.Request, msg string) {
	h.render(w, r, "challenges.html", map[string]interface{}{
		"Flash":     msg,
		"FlashType": "error",
	})
}

func extractPathSegment(path, prefix string) string {
	s := strings.TrimPrefix(path, prefix)
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx]
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func newCTFdDetector(url, apiKey string) *ctfd.Detector {
	return ctfd.NewDetector(url, apiKey)
}
