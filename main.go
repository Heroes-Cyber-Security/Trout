package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hcs.ctf/trout/internal"
	"hcs.ctf/trout/internal/config"
)

func main() {
	dbPath := flag.String("db", "trout.db", "path to sqlite database")
	httpAddr := flag.String("http-addr", ":8080", "address for admin ui and webhooks")
	internalAddr := flag.String("internal-addr", "127.0.0.1:9125", "address for internal flag api")
	adminPassword := flag.String("admin-password", "", "password for admin ui (or TROUT_ADMIN_PASSWORD env)")
	flag.Parse()

	pass := *adminPassword
	if pass == "" {
		pass = os.Getenv("TROUT_ADMIN_PASSWORD")
	}
	if pass == "" {
		slog.Error("admin password required via --admin-password or TROUT_ADMIN_PASSWORD env")
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slog.Info("opening database", "path", *dbPath)
	store, err := config.Open(*dbPath, pass)
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}

	svr := internal.New(store, pass)

	mainMux := svr.MainHandler()
	mainServer := &http.Server{
		Addr:    *httpAddr,
		Handler: mainMux,
	}

	internalMux := svr.InternalHandler()
	internalServer := &http.Server{
		Addr:    *internalAddr,
		Handler: internalMux,
	}

	svr.StartAllChallenges()

	go func() {
		slog.Info("main server listening", "addr", *httpAddr)
		if err := mainServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("main server error", "error", err)
			os.Exit(1)
		}
	}()

	go func() {
		slog.Info("internal server listening", "addr", *internalAddr)
		if err := internalServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("internal server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutting down", "signal", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mainServer.Shutdown(ctx)
	internalServer.Shutdown(ctx)
	svr.Shutdown()
	slog.Info("shutdown complete")
}
