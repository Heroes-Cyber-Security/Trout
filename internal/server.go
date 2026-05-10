package internal

import (
	"log/slog"
	"net/http"
	"strings"

	"hcs.ctf/trout/internal/api"
	"hcs.ctf/trout/internal/config"
	"hcs.ctf/trout/internal/ctfd"
	"hcs.ctf/trout/internal/discord"
	"hcs.ctf/trout/internal/flag"
	"hcs.ctf/trout/internal/netcat"
	"hcs.ctf/trout/internal/ui"
)

type Server struct {
	Admin    *ui.AdminHandler
	Internal *api.InternalHandler
	CTFdWH   *ctfd.WebhookHandler
	SubAPI   *api.SubmissionsHandler
	ncMgr    *netcat.Manager
	store    *config.Store
	discord  *discord.Notifier
	log      *slog.Logger
}

func New(store *config.Store, adminPassword string) *Server {
	log := slog.With("component", "server")

	disc := discord.New(store)

	notifyFn := func(eventType string, fields map[string]string) {
		disc.Send(eventType, fields)
	}

	logEventFn := func(eventType, source string, payload interface{}) {
		store.LogEvent(eventType, source, payload)
	}

	ctfdCfg, _ := store.GetCTFdConfig()
	ctfdClient := ctfd.New(ctfdCfg.URL, ctfdCfg.APIKey)

	verifyFn := func(token string) (int, error) {
		return ctfdClient.VerifyToken(token)
	}

	genFlagFn := func(baseFlag string, userID int, challengeID string) string {
		return flag.Generate(baseFlag, flag.Seed(userID, challengeID), nil)
	}

	ncMgr := netcat.NewManager(store, verifyFn, genFlagFn, logEventFn, notifyFn)

	admin := ui.NewAdmin(store, adminPassword, ncMgr)

	chalLookup := func(id string) (string, bool) {
		c, err := store.GetChallenge(id)
		if err != nil || c == nil {
			return "", false
		}
		return c.BaseFlag, true
	}

	internalH := api.NewInternal(genFlagFn, chalLookup)

	ctfdWH := ctfd.NewWebhookHandler(ctfdCfg.WebhookSecret, notifyFn)

	subAPI := api.NewSubmissions(ctfdCfg.WebhookSecret, notifyFn)

	return &Server{
		Admin:    admin,
		Internal: internalH,
		CTFdWH:   ctfdWH,
		SubAPI:   subAPI,
		ncMgr:    ncMgr,
		store:    store,
		discord:  disc,
		log:      log,
	}
}

func (s *Server) MainHandler() http.Handler {
	mux := http.NewServeMux()

	auth := s.Admin.AuthMiddleware

	mux.HandleFunc("/admin/login", s.Admin.Login)
	mux.Handle("/admin/", auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case path == "/admin/" || path == "/admin":
			s.Admin.Dashboard(w, r)
		case path == "/admin/challenges" && r.Method == http.MethodPost:
			s.Admin.CreateChallenge(w, r)
		case path == "/admin/challenges" || path == "/admin/challenges/":
			s.Admin.ListChallenges(w, r)
		case path == "/admin/challenges/new":
			s.Admin.NewChallengeForm(w, r)
		case strings.HasSuffix(path, "/edit"):
			s.Admin.EditChallengeForm(w, r)
		case strings.HasSuffix(path, "/toggle"):
			s.Admin.ToggleChallenge(w, r)
		case strings.HasSuffix(path, "/delete"):
			s.Admin.DeleteChallenge(w, r)
		case strings.Count(strings.TrimPrefix(path, "/admin/challenges/"), "/") == 0:
			s.Admin.ViewChallenge(w, r)
		case path == "/admin/settings/ctfd":
			s.Admin.CTFdSettings(w, r)
		case path == "/admin/settings/discord":
			s.Admin.DiscordSettings(w, r)
		case path == "/admin/logs":
			s.Admin.Logs(w, r)
		default:
			http.NotFound(w, r)
		}
	})))

	mux.Handle("/ctfd/webhook", s.CTFdWH)
	mux.Handle("/api/v1/submissions", s.SubAPI)

	return mux
}

func (s *Server) InternalHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/internal/flag", s.Internal)
	return mux
}

func (s *Server) StartAllChallenges() {
	chals, err := s.store.ListChallenges()
	if err != nil {
		s.log.Error("list challenges", "error", err)
		return
	}
	for _, c := range chals {
		if c.Enabled {
			if err := s.ncMgr.Start(c); err != nil {
				s.log.Error("start challenge", "id", c.ID, "error", err)
			}
		}
	}
}

func (s *Server) Shutdown() {
	s.ncMgr.StopAll()
	s.store.Close()
}
