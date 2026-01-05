// Package config provides configuration for the IPTV proxy.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Config holds the application configuration.
type Config struct {
	// Required
	M3UURL  string
	EPGURL  string
	BaseURL string

	// Server
	BindAddr string
	Port     int
	LogLevel string

	// HDHomeRun
	TunerCount int
	DeviceID   string
	DeviceName string

	// Data refresh
	RefreshInterval time.Duration
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		BindAddr:        "0.0.0.0",
		Port:            8080,
		LogLevel:        "info",
		TunerCount:      2,
		DeviceID:        "iptv-proxy-001",
		DeviceName:      "IPTV-Proxy",
		RefreshInterval: 30 * time.Minute,
	}
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if c.M3UURL == "" {
		return errors.New("--m3u is required")
	}

	if _, err := url.Parse(c.M3UURL); err != nil {
		return fmt.Errorf("invalid M3U URL: %w", err)
	}

	if c.EPGURL == "" {
		return errors.New("--epg is required")
	}

	epgURLs := c.EPGURLs()
	if len(epgURLs) == 0 {
		return errors.New("--epg must contain at least one valid URL")
	}

	for i, epgURL := range epgURLs {
		if _, err := url.Parse(epgURL); err != nil {
			return fmt.Errorf("invalid EPG URL at position %d: %w", i+1, err)
		}
	}

	if c.BaseURL == "" {
		return errors.New("--base is required")
	}

	if _, err := url.Parse(c.BaseURL); err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}

	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Port)
	}

	if c.TunerCount < 1 {
		return errors.New("tuner count must be at least 1")
	}

	return nil
}

// ListenAddr returns the full listen address.
func (c *Config) ListenAddr() string {
	return fmt.Sprintf("%s:%d", c.BindAddr, c.Port)
}

// EPGURLs returns the list of EPG URLs (comma-separated in EPGURL).
func (c *Config) EPGURLs() []string {
	if c.EPGURL == "" {
		return nil
	}

	urls := strings.Split(c.EPGURL, ",")
	result := make([]string, 0, len(urls))

	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u != "" {
			result = append(result, u)
		}
	}

	return result
}
