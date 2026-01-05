package hdhr

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/savid/iptv/internal/config"
	"github.com/savid/iptv/internal/data"
	"github.com/savid/iptv/internal/m3u"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func newTestLogger() logrus.FieldLogger {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	return logger
}

func newTestConfig() *config.Config {
	cfg := config.DefaultConfig()
	cfg.BaseURL = "http://localhost:8080"
	cfg.TunerCount = 2
	cfg.DeviceID = "test-device-001"

	return cfg
}

func TestNewHandlers(t *testing.T) {
	log := newTestLogger()
	cfg := newTestConfig()
	store := data.NewStore()

	handlers := NewHandlers(log, cfg, store)

	require.NotNil(t, handlers)
}

func TestRootXML_ValidResponse(t *testing.T) {
	log := newTestLogger()
	cfg := newTestConfig()
	store := data.NewStore()
	handlers := NewHandlers(log, cfg, store)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handlers.RootXML(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Contains(t, string(body), `<?xml version="1.0" encoding="UTF-8"?>`)
	require.Contains(t, string(body), "urn:schemas-upnp-org:device-1-0")
	require.Contains(t, string(body), "IPTV-Proxy")
	require.Contains(t, string(body), "Silicondust")
	require.Contains(t, string(body), "HDTC-2US")
	require.Contains(t, string(body), cfg.DeviceID)
	require.Contains(t, string(body), cfg.BaseURL)
}

func TestRootXML_ContentType(t *testing.T) {
	log := newTestLogger()
	cfg := newTestConfig()
	store := data.NewStore()
	handlers := NewHandlers(log, cfg, store)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handlers.RootXML(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	require.Equal(t, "application/xml", resp.Header.Get("Content-Type"))
}

func TestRootXML_ValidXMLStructure(t *testing.T) {
	log := newTestLogger()
	cfg := newTestConfig()
	store := data.NewStore()
	handlers := NewHandlers(log, cfg, store)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handlers.RootXML(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var device DeviceXML

	err = xml.Unmarshal(body, &device)
	require.NoError(t, err)

	require.Equal(t, cfg.BaseURL, device.URLBase)
	require.Equal(t, 1, device.SpecVersion.Major)
	require.Equal(t, 0, device.SpecVersion.Minor)
	require.Equal(t, "IPTV-Proxy", device.Device.FriendlyName)
	require.Equal(t, cfg.DeviceID, device.Device.SerialNumber)
}

func TestDiscovery_ValidJSON(t *testing.T) {
	log := newTestLogger()
	cfg := newTestConfig()
	store := data.NewStore()
	handlers := NewHandlers(log, cfg, store)

	req := httptest.NewRequest(http.MethodGet, "/discover.json", nil)
	w := httptest.NewRecorder()

	handlers.Discovery(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var discovery DiscoveryJSON

	err := json.NewDecoder(resp.Body).Decode(&discovery)
	require.NoError(t, err)

	require.Equal(t, "IPTV-Proxy", discovery.FriendlyName)
	require.Equal(t, "Golang", discovery.Manufacturer)
	require.Equal(t, cfg.TunerCount, discovery.TunerCount)
	require.Equal(t, cfg.DeviceID, discovery.DeviceID)
	require.Equal(t, cfg.BaseURL, discovery.BaseURL)
	require.Equal(t, cfg.BaseURL+"/lineup.json", discovery.LineupURL)
}

func TestDiscovery_TunerCount(t *testing.T) {
	tests := []struct {
		name       string
		tunerCount int
	}{
		{"one tuner", 1},
		{"two tuners", 2},
		{"four tuners", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := newTestLogger()
			cfg := newTestConfig()
			cfg.TunerCount = tt.tunerCount
			store := data.NewStore()
			handlers := NewHandlers(log, cfg, store)

			req := httptest.NewRequest(http.MethodGet, "/discover.json", nil)
			w := httptest.NewRecorder()

			handlers.Discovery(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			var discovery DiscoveryJSON

			err := json.NewDecoder(resp.Body).Decode(&discovery)
			require.NoError(t, err)
			require.Equal(t, tt.tunerCount, discovery.TunerCount)
		})
	}
}

func TestLineup_ValidJSON(t *testing.T) {
	log := newTestLogger()
	cfg := newTestConfig()
	store := data.NewStore()

	channels := []m3u.Channel{
		{Name: "ESPN", URL: "http://stream.example.com/1"},
		{Name: "HBO", URL: "http://stream.example.com/2"},
		{Name: "CNN", URL: "http://stream.example.com/3"},
	}
	store.SetM3U(channels)

	handlers := NewHandlers(log, cfg, store)

	req := httptest.NewRequest(http.MethodGet, "/lineup.json", nil)
	w := httptest.NewRecorder()

	handlers.Lineup(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var lineup []LineupItem

	err := json.NewDecoder(resp.Body).Decode(&lineup)
	require.NoError(t, err)

	require.Len(t, lineup, 3)
	require.Equal(t, "1", lineup[0].GuideNumber)
	require.Equal(t, "ESPN", lineup[0].GuideName)
	require.Equal(t, "http://stream.example.com/1", lineup[0].URL)
	require.Equal(t, "2", lineup[1].GuideNumber)
	require.Equal(t, "HBO", lineup[1].GuideName)
	require.Equal(t, "3", lineup[2].GuideNumber)
	require.Equal(t, "CNN", lineup[2].GuideName)
}

func TestLineup_NoData(t *testing.T) {
	log := newTestLogger()
	cfg := newTestConfig()
	store := data.NewStore()
	handlers := NewHandlers(log, cfg, store)

	req := httptest.NewRequest(http.MethodGet, "/lineup.json", nil)
	w := httptest.NewRecorder()

	handlers.Lineup(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestLineup_ChannelNumbering(t *testing.T) {
	log := newTestLogger()
	cfg := newTestConfig()
	store := data.NewStore()

	channels := make([]m3u.Channel, 10)
	for i := 0; i < 10; i++ {
		channels[i] = m3u.Channel{Name: "Channel", URL: "http://example.com"}
	}

	store.SetM3U(channels)

	handlers := NewHandlers(log, cfg, store)

	req := httptest.NewRequest(http.MethodGet, "/lineup.json", nil)
	w := httptest.NewRecorder()

	handlers.Lineup(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	var lineup []LineupItem

	err := json.NewDecoder(resp.Body).Decode(&lineup)
	require.NoError(t, err)

	require.Len(t, lineup, 10)
	require.Equal(t, "1", lineup[0].GuideNumber)
	require.Equal(t, "2", lineup[1].GuideNumber)
	require.Equal(t, "5", lineup[4].GuideNumber)
	require.Equal(t, "10", lineup[9].GuideNumber)
}

func TestLineupStatus_ValidJSON(t *testing.T) {
	log := newTestLogger()
	cfg := newTestConfig()
	store := data.NewStore()
	handlers := NewHandlers(log, cfg, store)

	req := httptest.NewRequest(http.MethodGet, "/lineup_status.json", nil)
	w := httptest.NewRecorder()

	handlers.LineupStatus(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var status LineupStatus

	err := json.NewDecoder(resp.Body).Decode(&status)
	require.NoError(t, err)

	require.Equal(t, 0, status.ScanInProgress)
	require.Equal(t, 0, status.ScanPossible)
	require.Equal(t, "Cable", status.Source)
	require.Equal(t, []string{"Cable"}, status.SourceList)
}

func TestAutoTune_ValidChannel(t *testing.T) {
	log := newTestLogger()
	cfg := newTestConfig()
	store := data.NewStore()

	channels := []m3u.Channel{
		{Name: "ESPN", URL: "http://stream.example.com/espn"},
		{Name: "HBO", URL: "http://stream.example.com/hbo"},
	}
	store.SetM3U(channels)

	handlers := NewHandlers(log, cfg, store)

	tests := []struct {
		name        string
		path        string
		expectedURL string
	}{
		{"channel 1", "/auto/v1", "http://stream.example.com/espn"},
		{"channel 2", "/auto/v2", "http://stream.example.com/hbo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			handlers.AutoTune(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			require.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode)
			require.Equal(t, tt.expectedURL, resp.Header.Get("Location"))
		})
	}
}

func TestAutoTune_InvalidChannel(t *testing.T) {
	log := newTestLogger()
	cfg := newTestConfig()
	store := data.NewStore()

	channels := []m3u.Channel{
		{Name: "ESPN", URL: "http://stream.example.com/espn"},
	}
	store.SetM3U(channels)

	handlers := NewHandlers(log, cfg, store)

	tests := []struct {
		name string
		path string
	}{
		{"too short path", "/auto/v"},
		{"non-numeric", "/auto/vabc"},
		{"empty after v", "/auto/v"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			handlers.AutoTune(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})
	}
}

func TestAutoTune_ChannelNotFound(t *testing.T) {
	log := newTestLogger()
	cfg := newTestConfig()
	store := data.NewStore()

	channels := []m3u.Channel{
		{Name: "ESPN", URL: "http://stream.example.com/espn"},
	}
	store.SetM3U(channels)

	handlers := NewHandlers(log, cfg, store)

	tests := []struct {
		name string
		path string
	}{
		{"channel 0", "/auto/v0"},
		{"channel 2 when only 1 exists", "/auto/v2"},
		{"channel 100", "/auto/v100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			handlers.AutoTune(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			require.Equal(t, http.StatusNotFound, resp.StatusCode)
		})
	}
}

func TestAutoTune_NoData(t *testing.T) {
	log := newTestLogger()
	cfg := newTestConfig()
	store := data.NewStore()
	handlers := NewHandlers(log, cfg, store)

	req := httptest.NewRequest(http.MethodGet, "/auto/v1", nil)
	w := httptest.NewRecorder()

	handlers.AutoTune(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestAutoTune_LargeChannelNumber(t *testing.T) {
	log := newTestLogger()
	cfg := newTestConfig()
	store := data.NewStore()

	channels := make([]m3u.Channel, 500)
	for i := 0; i < 500; i++ {
		channels[i] = m3u.Channel{Name: "Channel", URL: "http://example.com/" + string(rune(i))}
	}

	channels[499].URL = "http://stream.example.com/channel500"
	store.SetM3U(channels)

	handlers := NewHandlers(log, cfg, store)

	req := httptest.NewRequest(http.MethodGet, "/auto/v500", nil)
	w := httptest.NewRecorder()

	handlers.AutoTune(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode)
	require.Equal(t, "http://stream.example.com/channel500", resp.Header.Get("Location"))
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"US Sports", "us-sports"},
		{"UK Movies", "uk-movies"},
		{"News", "news"},
		{"US_Entertainment", "us-entertainment"},
		{"UK: Drama", "uk-drama"},
		{"Sports & News", "sports-news"},
		{"  Spaces  ", "spaces"},
		{"MixedCase", "mixedcase"},
		{"123 Numbers", "123-numbers"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := Slugify(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestNewGroupHandlers(t *testing.T) {
	log := newTestLogger()
	cfg := newTestConfig()
	store := data.NewStore()

	handlers := NewGroupHandlers(log, cfg, store, "US Sports")

	require.Equal(t, "iptv-us-sports", handlers.DeviceID())
}

func TestGroupHandlers_Lineup(t *testing.T) {
	log := newTestLogger()
	cfg := newTestConfig()
	store := data.NewStore()

	channels := []m3u.Channel{
		{Name: "ESPN", URL: "http://stream.example.com/espn", Group: "Sports"},
		{Name: "Fox Sports", URL: "http://stream.example.com/fox", Group: "Sports"},
		{Name: "HBO", URL: "http://stream.example.com/hbo", Group: "Movies"},
		{Name: "CNN", URL: "http://stream.example.com/cnn", Group: "News"},
	}
	store.SetM3U(channels)

	handlers := NewGroupHandlers(log, cfg, store, "Sports")

	req := httptest.NewRequest(http.MethodGet, "/sports/lineup.json", nil)
	w := httptest.NewRecorder()

	handlers.Lineup(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var lineup []LineupItem

	err := json.NewDecoder(resp.Body).Decode(&lineup)
	require.NoError(t, err)

	require.Len(t, lineup, 2)
	require.Equal(t, "1", lineup[0].GuideNumber)
	require.Equal(t, "ESPN", lineup[0].GuideName)
	require.Equal(t, "2", lineup[1].GuideNumber)
	require.Equal(t, "Fox Sports", lineup[1].GuideName)
}

func TestGroupHandlers_Discovery(t *testing.T) {
	log := newTestLogger()
	cfg := newTestConfig()
	store := data.NewStore()

	handlers := NewGroupHandlers(log, cfg, store, "US Sports")

	req := httptest.NewRequest(http.MethodGet, "/us-sports/discover.json", nil)
	w := httptest.NewRecorder()

	handlers.Discovery(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var discovery DiscoveryJSON

	err := json.NewDecoder(resp.Body).Decode(&discovery)
	require.NoError(t, err)

	require.Equal(t, "IPTV-Proxy (US Sports)", discovery.FriendlyName)
	require.Equal(t, "iptv-us-sports", discovery.DeviceID)
	require.Equal(t, "http://localhost:8080/us-sports", discovery.BaseURL)
	require.Equal(t, "http://localhost:8080/us-sports/lineup.json", discovery.LineupURL)
}

func TestGroupHandlers_AutoTune(t *testing.T) {
	log := newTestLogger()
	cfg := newTestConfig()
	store := data.NewStore()

	channels := []m3u.Channel{
		{Name: "ESPN", URL: "http://stream.example.com/espn", Group: "Sports"},
		{Name: "Fox Sports", URL: "http://stream.example.com/fox", Group: "Sports"},
		{Name: "HBO", URL: "http://stream.example.com/hbo", Group: "Movies"},
	}
	store.SetM3U(channels)

	handlers := NewGroupHandlers(log, cfg, store, "Sports")

	req := httptest.NewRequest(http.MethodGet, "/sports/auto/v2", nil)
	w := httptest.NewRecorder()

	handlers.AutoTune(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode)
	require.Equal(t, "http://stream.example.com/fox", resp.Header.Get("Location"))
}

func TestGroupHandlers_AutoTune_OutOfRange(t *testing.T) {
	log := newTestLogger()
	cfg := newTestConfig()
	store := data.NewStore()

	channels := []m3u.Channel{
		{Name: "ESPN", URL: "http://stream.example.com/espn", Group: "Sports"},
		{Name: "HBO", URL: "http://stream.example.com/hbo", Group: "Movies"},
	}
	store.SetM3U(channels)

	handlers := NewGroupHandlers(log, cfg, store, "Sports")

	req := httptest.NewRequest(http.MethodGet, "/sports/auto/v2", nil)
	w := httptest.NewRecorder()

	handlers.AutoTune(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}
