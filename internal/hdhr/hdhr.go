// Package hdhr provides HDHomeRun emulation for Plex Live TV discovery.
package hdhr

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/savid/iptv/internal/config"
	"github.com/savid/iptv/internal/data"
	"github.com/sirupsen/logrus"
)

// DeviceXML represents the UPnP device description.
type DeviceXML struct {
	XMLName     xml.Name `xml:"root"`
	Xmlns       string   `xml:"xmlns,attr"`
	URLBase     string   `xml:"URLBase"`
	SpecVersion struct {
		Major int `xml:"major"`
		Minor int `xml:"minor"`
	} `xml:"specVersion"`
	Device struct {
		DeviceType   string `xml:"deviceType"`
		FriendlyName string `xml:"friendlyName"`
		Manufacturer string `xml:"manufacturer"`
		ModelName    string `xml:"modelName"`
		ModelNumber  string `xml:"modelNumber"`
		SerialNumber string `xml:"serialNumber"`
		UDN          string `xml:"UDN"`
	} `xml:"device"`
}

// DiscoveryJSON represents the device discovery response.
// JSON field names use PascalCase as required by HDHomeRun protocol.
//
//nolint:tagliatelle // HDHomeRun protocol requires PascalCase JSON field names
type DiscoveryJSON struct {
	FriendlyName    string `json:"FriendlyName"`
	Manufacturer    string `json:"Manufacturer"`
	ManufacturerURL string `json:"ManufacturerURL"`
	ModelNumber     string `json:"ModelNumber"`
	FirmwareName    string `json:"FirmwareName"`
	TunerCount      int    `json:"TunerCount"`
	FirmwareVersion string `json:"FirmwareVersion"`
	DeviceID        string `json:"DeviceID"`
	DeviceAuth      string `json:"DeviceAuth"`
	BaseURL         string `json:"BaseURL"`
	LineupURL       string `json:"LineupURL"`
}

// LineupItem represents a channel in the lineup.
// JSON field names use PascalCase as required by HDHomeRun protocol.
//
//nolint:tagliatelle // HDHomeRun protocol requires PascalCase JSON field names
type LineupItem struct {
	GuideNumber string `json:"GuideNumber"`
	GuideName   string `json:"GuideName"`
	URL         string `json:"URL"`
}

// LineupStatus represents the lineup scanning status.
// JSON field names use PascalCase as required by HDHomeRun protocol.
//
//nolint:tagliatelle // HDHomeRun protocol requires PascalCase JSON field names
type LineupStatus struct {
	ScanInProgress int      `json:"ScanInProgress"`
	ScanPossible   int      `json:"ScanPossible"`
	Source         string   `json:"Source"`
	SourceList     []string `json:"SourceList"`
}

// Handlers provides HTTP handlers for HDHomeRun emulation.
type Handlers struct {
	log      logrus.FieldLogger
	cfg      *config.Config
	store    *data.Store
	group    string // Group name filter (empty = all channels)
	deviceID string // Unique device ID for this handler
	baseURL  string // Base URL including group path prefix
}

// NewHandlers creates a new HDHomeRun handlers instance for all channels (root device).
func NewHandlers(log logrus.FieldLogger, cfg *config.Config, store *data.Store) *Handlers {
	return &Handlers{
		log:      log.WithField("component", "hdhr"),
		cfg:      cfg,
		store:    store,
		group:    "",
		deviceID: cfg.DeviceID,
		baseURL:  cfg.BaseURL,
	}
}

// NewGroupHandlers creates a new HDHomeRun handlers instance for a specific group.
func NewGroupHandlers(log logrus.FieldLogger, cfg *config.Config, store *data.Store, group string) *Handlers {
	slug := Slugify(group)

	return &Handlers{
		log:      log.WithFields(logrus.Fields{"component": "hdhr", "group": group}),
		cfg:      cfg,
		store:    store,
		group:    group,
		deviceID: fmt.Sprintf("iptv-%s", slug),
		baseURL:  fmt.Sprintf("%s/%s", cfg.BaseURL, slug),
	}
}

// DeviceID returns the device ID for this handler.
func (h *Handlers) DeviceID() string {
	return h.deviceID
}

// Slugify converts a group name to a URL-safe slug.
// Example: "US Sports" -> "us-sports".
func Slugify(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")

	reg := regexp.MustCompile(`[^a-z0-9-]+`)
	s = reg.ReplaceAllString(s, "")

	// Collapse multiple hyphens and trim leading/trailing hyphens
	reg = regexp.MustCompile(`-+`)
	s = reg.ReplaceAllString(s, "-")

	return strings.Trim(s, "-")
}

// RootXML serves the UPnP device description at /.
func (h *Handlers) RootXML(w http.ResponseWriter, _ *http.Request) {
	friendlyName := h.cfg.DeviceName
	if h.group != "" {
		friendlyName = fmt.Sprintf("%s (%s)", h.cfg.DeviceName, h.group)
	}

	device := DeviceXML{
		Xmlns:   "urn:schemas-upnp-org:device-1-0",
		URLBase: h.baseURL,
	}
	device.SpecVersion.Major = 1
	device.SpecVersion.Minor = 0
	device.Device.DeviceType = "urn:schemas-upnp-org:device:MediaServer:1"
	device.Device.FriendlyName = friendlyName
	device.Device.Manufacturer = "Silicondust"
	device.Device.ModelName = "HDTC-2US"
	device.Device.ModelNumber = "HDTC-2US"
	device.Device.SerialNumber = h.deviceID
	device.Device.UDN = fmt.Sprintf("uuid:%s", h.deviceID)

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte(xml.Header)); err != nil {
		h.log.WithError(err).Error("Failed to write XML header")

		return
	}

	encoder := xml.NewEncoder(w)
	encoder.Indent("", "  ")

	if err := encoder.Encode(device); err != nil {
		h.log.WithError(err).Error("Failed to encode device XML")

		return
	}
}

// Discovery serves device discovery JSON at /discover.json and /discovery.json.
func (h *Handlers) Discovery(w http.ResponseWriter, _ *http.Request) {
	friendlyName := h.cfg.DeviceName
	if h.group != "" {
		friendlyName = fmt.Sprintf("%s (%s)", h.cfg.DeviceName, h.group)
	}

	discovery := DiscoveryJSON{
		FriendlyName:    friendlyName,
		Manufacturer:    "Golang",
		ManufacturerURL: "https://github.com/savid/iptv",
		ModelNumber:     "1.0",
		FirmwareName:    "bin_1.0",
		TunerCount:      h.cfg.TunerCount,
		FirmwareVersion: "1.0",
		DeviceID:        h.deviceID,
		DeviceAuth:      "iptv-proxy",
		BaseURL:         h.baseURL,
		LineupURL:       fmt.Sprintf("%s/lineup.json", h.baseURL),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(discovery); err != nil {
		h.log.WithError(err).Error("Failed to encode discovery JSON")

		return
	}
}

// Lineup serves channel lineup at /lineup.json.
func (h *Handlers) Lineup(w http.ResponseWriter, _ *http.Request) {
	channels, ok := h.store.GetChannelsByGroup(h.group)
	if !ok || len(channels) == 0 {
		http.Error(w, "No channels available", http.StatusServiceUnavailable)

		return
	}

	lineup := make([]LineupItem, 0, len(channels))

	// Track name occurrences to suffix duplicates
	nameCount := make(map[string]int, len(channels))

	for i, channel := range channels {
		guideName := channel.Name

		// If we've seen this name before, suffix it
		if count := nameCount[channel.Name]; count > 0 {
			guideName = fmt.Sprintf("%s (%d)", channel.Name, count+1)
		}

		nameCount[channel.Name]++

		lineup = append(lineup, LineupItem{
			GuideNumber: fmt.Sprintf("%d", i+1),
			GuideName:   guideName,
			URL:         channel.URL,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(lineup); err != nil {
		h.log.WithError(err).Error("Failed to encode lineup JSON")

		return
	}
}

// LineupStatus serves the lineup scanning status at /lineup_status.json.
func (h *Handlers) LineupStatus(w http.ResponseWriter, _ *http.Request) {
	status := LineupStatus{
		ScanInProgress: 0,
		ScanPossible:   0,
		Source:         "Cable",
		SourceList:     []string{"Cable"},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(status); err != nil {
		h.log.WithError(err).Error("Failed to encode lineup status JSON")

		return
	}
}

// AutoTune handles HDHomeRun-style tuning URLs at /auto/v{channel}.
// This redirects to the upstream URL for the requested channel.
func (h *Handlers) AutoTune(w http.ResponseWriter, r *http.Request) {
	// Extract channel number from path: /auto/v{channel} or /{group}/auto/v{channel}
	path := r.URL.Path

	// Find the position of "/auto/v" in the path
	autoIdx := strings.Index(path, "/auto/v")
	if autoIdx == -1 || len(path) <= autoIdx+7 {
		http.Error(w, "Invalid channel", http.StatusBadRequest)

		return
	}

	channelNum := path[autoIdx+7:] // Everything after "/auto/v"

	channels, ok := h.store.GetChannelsByGroup(h.group)
	if !ok || len(channels) == 0 {
		http.Error(w, "No channels available", http.StatusServiceUnavailable)

		return
	}

	// Find channel by number (1-indexed)
	var channelIdx int
	if _, err := fmt.Sscanf(channelNum, "%d", &channelIdx); err != nil {
		http.Error(w, "Invalid channel number", http.StatusBadRequest)

		return
	}

	if channelIdx < 1 || channelIdx > len(channels) {
		h.log.WithField("channel", channelIdx).Error("Channel not found")
		http.Error(w, "Channel not found", http.StatusNotFound)

		return
	}

	channel := channels[channelIdx-1]

	h.log.WithFields(logrus.Fields{
		"channel": channelIdx,
		"name":    channel.Name,
		"group":   h.group,
	}).Debug("AutoTune redirect")

	// Redirect directly to upstream URL
	http.Redirect(w, r, channel.URL, http.StatusTemporaryRedirect)
}
