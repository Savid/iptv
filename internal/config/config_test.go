package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	testM3UURL     = "http://example.com/playlist.m3u"
	testEPGURL     = "http://example.com/epg.xml"
	testBaseURL    = "http://localhost:8080"
	testInvalidURL = "://invalid-url"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	require.Equal(t, "0.0.0.0", cfg.BindAddr)
	require.Equal(t, 8080, cfg.Port)
	require.Equal(t, "info", cfg.LogLevel)
	require.Equal(t, 2, cfg.TunerCount)
	require.Equal(t, "iptv-proxy-001", cfg.DeviceID)
	require.Equal(t, 30*time.Minute, cfg.RefreshInterval)

	require.Empty(t, cfg.M3UURL)
	require.Empty(t, cfg.EPGURL)
	require.Empty(t, cfg.BaseURL)
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.M3UURL = testM3UURL
	cfg.EPGURL = testEPGURL
	cfg.BaseURL = testBaseURL

	err := cfg.Validate()
	require.NoError(t, err)
}

func TestValidate_MissingM3UURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EPGURL = testEPGURL
	cfg.BaseURL = testBaseURL

	err := cfg.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--m3u is required")
}

func TestValidate_MissingEPGURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.M3UURL = testM3UURL
	cfg.BaseURL = testBaseURL

	err := cfg.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--epg is required")
}

func TestValidate_MissingBaseURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.M3UURL = testM3UURL
	cfg.EPGURL = testEPGURL

	err := cfg.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--base is required")
}

func TestValidate_InvalidM3UURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.M3UURL = testInvalidURL
	cfg.EPGURL = testEPGURL
	cfg.BaseURL = testBaseURL

	err := cfg.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid M3U URL")
}

func TestValidate_InvalidEPGURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.M3UURL = testM3UURL
	cfg.EPGURL = testInvalidURL
	cfg.BaseURL = testBaseURL

	err := cfg.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid EPG URL")
}

func TestValidate_InvalidBaseURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.M3UURL = testM3UURL
	cfg.EPGURL = testEPGURL
	cfg.BaseURL = testInvalidURL

	err := cfg.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid base URL")
}

func TestValidate_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"port zero", 0},
		{"port negative", -1},
		{"port too high", 65536},
		{"port way too high", 100000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.M3UURL = testM3UURL
			cfg.EPGURL = testEPGURL
			cfg.BaseURL = testBaseURL
			cfg.Port = tt.port

			err := cfg.Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), "port must be between 1 and 65535")
		})
	}
}

func TestValidate_ValidPortBoundaries(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"port 1", 1},
		{"port 80", 80},
		{"port 8080", 8080},
		{"port 65535", 65535},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.M3UURL = testM3UURL
			cfg.EPGURL = testEPGURL
			cfg.BaseURL = testBaseURL
			cfg.Port = tt.port

			err := cfg.Validate()
			require.NoError(t, err)
		})
	}
}

func TestValidate_InvalidTunerCount(t *testing.T) {
	tests := []struct {
		name       string
		tunerCount int
	}{
		{"zero tuners", 0},
		{"negative tuners", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.M3UURL = testM3UURL
			cfg.EPGURL = testEPGURL
			cfg.BaseURL = testBaseURL
			cfg.TunerCount = tt.tunerCount

			err := cfg.Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), "tuner count must be at least 1")
		})
	}
}

func TestValidate_ValidTunerCount(t *testing.T) {
	tests := []struct {
		name       string
		tunerCount int
	}{
		{"one tuner", 1},
		{"two tuners", 2},
		{"many tuners", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.M3UURL = testM3UURL
			cfg.EPGURL = testEPGURL
			cfg.BaseURL = testBaseURL
			cfg.TunerCount = tt.tunerCount

			err := cfg.Validate()
			require.NoError(t, err)
		})
	}
}

func TestListenAddr(t *testing.T) {
	tests := []struct {
		name     string
		bindAddr string
		port     int
		expected string
	}{
		{
			name:     "default",
			bindAddr: "0.0.0.0",
			port:     8080,
			expected: "0.0.0.0:8080",
		},
		{
			name:     "localhost",
			bindAddr: "127.0.0.1",
			port:     3000,
			expected: "127.0.0.1:3000",
		},
		{
			name:     "custom bind",
			bindAddr: "192.168.1.100",
			port:     9000,
			expected: "192.168.1.100:9000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				BindAddr: tt.bindAddr,
				Port:     tt.port,
			}

			result := cfg.ListenAddr()
			require.Equal(t, tt.expected, result)
		})
	}
}
