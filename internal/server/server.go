package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/savid/iptv/internal/config"
	"github.com/savid/iptv/internal/data"
	"github.com/savid/iptv/internal/hdhr"
	"github.com/sirupsen/logrus"
)

const (
	readTimeout     = 10 * time.Second
	writeTimeout    = 0 // No timeout for streaming
	idleTimeout     = 120 * time.Second
	shutdownTimeout = 30 * time.Second
)

// Server provides the HTTP server with lifecycle management.
type Server struct {
	log       logrus.FieldLogger
	cfg       *config.Config
	store     *data.Store
	fetcher   *data.Fetcher
	refresher *data.Refresher
	server    *http.Server

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewServer creates a new server instance.
func NewServer(log logrus.FieldLogger, cfg *config.Config) *Server {
	store := data.NewStore()
	fetcher := data.NewFetcher(log, cfg.M3UURL, cfg.EPGURLs(), store)
	refresher := data.NewRefresher(log, fetcher, cfg.RefreshInterval)

	return &Server{
		log:       log.WithField("component", "server"),
		cfg:       cfg,
		store:     store,
		fetcher:   fetcher,
		refresher: refresher,
	}
}

// Start starts the server.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		return errors.New("server already running")
	}

	// Create cancellable context
	serverCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.done = make(chan struct{})

	// Fetch initial data
	s.log.Info("Fetching initial data")

	if err := s.fetcher.FetchAll(serverCtx); err != nil {
		cancel()

		return fmt.Errorf("failed to fetch initial data: %w", err)
	}

	// Start data refresher
	if err := s.refresher.Start(serverCtx); err != nil {
		cancel()

		return fmt.Errorf("failed to start refresher: %w", err)
	}

	// Start status logger
	go s.startStatusLogger(serverCtx)

	// Create routes
	routes := NewRoutes(s.log, s.cfg, s.store)

	// Create HTTP server
	s.server = &http.Server{
		Addr:         s.cfg.ListenAddr(),
		Handler:      routes.Handler(),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	// Start HTTP server
	go s.run(serverCtx)

	s.log.WithField("addr", s.cfg.ListenAddr()).Info("Server started")

	return nil
}

// Stop stops the server.
func (s *Server) Stop() error {
	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	s.cancel = nil
	s.done = nil
	s.mu.Unlock()

	if cancel == nil {
		return nil
	}

	// Cancel context
	cancel()

	// Wait for server to stop
	if done != nil {
		<-done
	}

	// Stop refresher
	if err := s.refresher.Stop(); err != nil {
		s.log.WithError(err).Warn("Failed to stop refresher")
	}

	s.log.Info("Server stopped")

	return nil
}

func (s *Server) run(ctx context.Context) {
	defer close(s.done)

	// Start HTTP server in goroutine
	errCh := make(chan error, 1)

	go func() {
		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}

		close(errCh)
	}()

	// Wait for shutdown signal or error
	select {
	case <-ctx.Done():
		s.log.Info("Shutting down server")
	case err := <-errCh:
		if err != nil {
			s.log.WithError(err).Error("Server error")
		}

		return
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := s.server.Shutdown(shutdownCtx); err != nil {
		s.log.WithError(err).Warn("Server shutdown error")
	}
}

// startStatusLogger logs available tuners every minute.
func (s *Server) startStatusLogger(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// Log immediately on start
	s.logTunerStatus()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.logTunerStatus()
		}
	}
}

func (s *Server) logTunerStatus() {
	channels, ok := s.store.GetM3U()
	if !ok {
		s.log.Warn("No M3U data available for status")

		return
	}

	s.log.Info("Available tuners:")

	// Root device (all channels)
	s.log.WithFields(logrus.Fields{
		"channels": len(channels),
		"url":      s.cfg.BaseURL + "/",
	}).Info("  All Channels")

	// Per-group devices
	groups := s.store.GetGroups()

	for _, group := range groups {
		groupChannels, _ := s.store.GetChannelsByGroup(group)
		slug := hdhr.Slugify(group)

		s.log.WithFields(logrus.Fields{
			"channels": len(groupChannels),
			"url":      fmt.Sprintf("%s/%s/", s.cfg.BaseURL, slug),
		}).Info("  " + group)
	}
}
