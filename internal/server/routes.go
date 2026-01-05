// Package server provides the HTTP server and routing.
package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/savid/iptv/internal/config"
	"github.com/savid/iptv/internal/data"
	"github.com/savid/iptv/internal/epg"
	"github.com/savid/iptv/internal/hdhr"
	"github.com/savid/iptv/internal/m3u"
	"github.com/sirupsen/logrus"
)

// Routes sets up all HTTP routes.
type Routes struct {
	log          logrus.FieldLogger
	cfg          *config.Config
	store        *data.Store
	hdhrHandlers *hdhr.Handlers

	// Group handlers are created dynamically based on M3U data.
	groupHandlersMu sync.RWMutex
	groupHandlers   map[string]*hdhr.Handlers // slug -> handlers
}

// NewRoutes creates a new routes instance.
func NewRoutes(
	log logrus.FieldLogger,
	cfg *config.Config,
	store *data.Store,
) *Routes {
	return &Routes{
		log:           log.WithField("component", "routes"),
		cfg:           cfg,
		store:         store,
		hdhrHandlers:  hdhr.NewHandlers(log, cfg, store),
		groupHandlers: make(map[string]*hdhr.Handlers),
	}
}

// Handler returns the main HTTP handler with all routes.
func (r *Routes) Handler() http.Handler {
	mux := http.NewServeMux()

	// Root HDHomeRun emulation endpoints (all channels)
	mux.HandleFunc("/discover.json", r.hdhrHandlers.Discovery)
	mux.HandleFunc("/discovery.json", r.hdhrHandlers.Discovery)
	mux.HandleFunc("/lineup.json", r.hdhrHandlers.Lineup)
	mux.HandleFunc("/lineup_status.json", r.hdhrHandlers.LineupStatus)
	mux.HandleFunc("/auto/", r.hdhrHandlers.AutoTune)

	// Data endpoints
	mux.HandleFunc("/iptv.m3u", r.handleM3U)
	mux.HandleFunc("/epg.xml", r.handleEPG)

	// Health check
	mux.HandleFunc("/health", r.handleHealth)

	// Catch-all for root XML and group routes
	mux.HandleFunc("/", r.handleRootOrGroup)

	// Wrap with logging middleware
	return r.loggingMiddleware(mux)
}

// handleRootOrGroup handles the root path and dynamically routes to group handlers.
func (r *Routes) handleRootOrGroup(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path

	// Exact root path serves root device XML
	if path == "/" {
		r.hdhrHandlers.RootXML(w, req)

		return
	}

	// Try to match group routes: /{slug}/...
	// Remove leading slash and get the first path segment
	trimmed := strings.TrimPrefix(path, "/")
	parts := strings.SplitN(trimmed, "/", 2)

	if len(parts) == 0 {
		http.NotFound(w, req)

		return
	}

	slug := parts[0]
	remainder := ""

	if len(parts) > 1 {
		remainder = parts[1]
	}

	// Get or create handler for this group
	handler := r.getGroupHandler(slug)
	if handler == nil {
		http.NotFound(w, req)

		return
	}

	// Route to appropriate handler method based on remainder
	switch {
	case remainder == "" || remainder == "/":
		handler.RootXML(w, req)
	case remainder == "discover.json":
		handler.Discovery(w, req)
	case remainder == "discovery.json":
		handler.Discovery(w, req)
	case remainder == "lineup.json":
		handler.Lineup(w, req)
	case remainder == "lineup_status.json":
		handler.LineupStatus(w, req)
	case strings.HasPrefix(remainder, "auto/"):
		handler.AutoTune(w, req)
	default:
		http.NotFound(w, req)
	}
}

// getGroupHandler returns the handler for a group slug, creating it if necessary.
func (r *Routes) getGroupHandler(slug string) *hdhr.Handlers {
	// Check cache first
	r.groupHandlersMu.RLock()

	if handler, ok := r.groupHandlers[slug]; ok {
		r.groupHandlersMu.RUnlock()

		return handler
	}

	r.groupHandlersMu.RUnlock()

	// Find the group name that matches this slug
	groups := r.store.GetGroups()

	var groupName string

	for _, g := range groups {
		if hdhr.Slugify(g) == slug {
			groupName = g

			break
		}
	}

	if groupName == "" {
		return nil
	}

	// Create and cache the handler
	r.groupHandlersMu.Lock()
	defer r.groupHandlersMu.Unlock()

	// Double-check after acquiring write lock
	if handler, ok := r.groupHandlers[slug]; ok {
		return handler
	}

	handler := hdhr.NewGroupHandlers(r.log, r.cfg, r.store, groupName)
	r.groupHandlers[slug] = handler

	r.log.WithFields(logrus.Fields{
		"group":    groupName,
		"slug":     slug,
		"deviceID": handler.DeviceID(),
	}).Info("Created group tuner handler")

	return handler
}

func (r *Routes) handleM3U(w http.ResponseWriter, req *http.Request) {
	channels, ok := r.store.GetM3U()
	if !ok {
		http.Error(w, "No M3U data available", http.StatusServiceUnavailable)

		return
	}

	_, channelMap, _ := r.store.GetEPG()

	rewritten := m3u.Rewrite(channels, channelMap)

	w.Header().Set("Content-Type", "application/x-mpegurl")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte(rewritten)); err != nil {
		r.log.WithError(err).Error("Failed to write M3U response")
	}
}

func (r *Routes) handleEPG(w http.ResponseWriter, req *http.Request) {
	epgData, _, ok := r.store.GetEPG()
	if !ok {
		http.Error(w, "No EPG data available", http.StatusServiceUnavailable)

		return
	}

	xmlData, err := epg.Marshal(epgData)
	if err != nil {
		r.log.WithError(err).Error("Failed to marshal EPG")
		http.Error(w, "Failed to generate EPG", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(xmlData); err != nil {
		r.log.WithError(err).Error("Failed to write EPG response")
	}
}

func (r *Routes) handleHealth(w http.ResponseWriter, req *http.Request) {
	status := struct {
		Status   string `json:"status"`
		HasData  bool   `json:"hasData"`
		LastSync string `json:"lastSync"`
	}{
		Status:   "ok",
		HasData:  r.store.HasData(),
		LastSync: r.store.LastSync().Format("2006-01-02T15:04:05Z"),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(status); err != nil {
		r.log.WithError(err).Error("Failed to write health response")
	}
}

func (r *Routes) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.log.WithFields(logrus.Fields{
			"method": req.Method,
			"path":   req.URL.Path,
			"remote": req.RemoteAddr,
		}).Info("HTTP request")

		next.ServeHTTP(w, req)
	})
}
