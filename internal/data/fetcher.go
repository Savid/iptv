package data

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/savid/iptv/internal/epg"
	"github.com/savid/iptv/internal/m3u"
	"github.com/sirupsen/logrus"
)

const (
	defaultTimeout = 5 * time.Minute
	maxBodySize    = 500 * 1024 * 1024 // 500MB for large EPG files
)

// Fetcher fetches M3U and EPG data from remote URLs.
type Fetcher struct {
	log        logrus.FieldLogger
	httpClient *http.Client
	m3uURL     string
	epgURLs    []string
	store      *Store
}

// NewFetcher creates a new data fetcher.
func NewFetcher(log logrus.FieldLogger, m3uURL string, epgURLs []string, store *Store) *Fetcher {
	return &Fetcher{
		log: log.WithField("component", "fetcher"),
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		m3uURL:  m3uURL,
		epgURLs: epgURLs,
		store:   store,
	}
}

// FetchAll fetches both M3U and EPG data.
func (f *Fetcher) FetchAll(ctx context.Context) error {
	if err := f.FetchM3U(ctx); err != nil {
		return fmt.Errorf("failed to fetch M3U: %w", err)
	}

	if err := f.FetchEPG(ctx); err != nil {
		return fmt.Errorf("failed to fetch EPG: %w", err)
	}

	return nil
}

// FetchM3U fetches and parses the M3U playlist.
func (f *Fetcher) FetchM3U(ctx context.Context) error {
	f.log.WithField("url", f.m3uURL).Info("Fetching M3U playlist")

	data, err := f.fetch(ctx, f.m3uURL)
	if err != nil {
		return fmt.Errorf("failed to fetch M3U: %w", err)
	}

	channels, err := m3u.Parse(data)
	if err != nil {
		return fmt.Errorf("failed to parse M3U: %w", err)
	}

	f.store.SetM3U(channels)
	f.log.WithField("channels", len(channels)).Info("M3U playlist loaded")

	f.logGroupSummary(channels)

	return nil
}

// logGroupSummary logs a summary of channels per group.
func (f *Fetcher) logGroupSummary(channels []m3u.Channel) {
	groupCounts := make(map[string]int, 32)

	for _, ch := range channels {
		group := ch.Group
		if group == "" {
			group = "(no group)"
		}

		groupCounts[group]++
	}

	f.log.WithField("groups", len(groupCounts)).Info("Channel groups summary")

	for group, count := range groupCounts {
		f.log.WithFields(logrus.Fields{
			"group":    group,
			"channels": count,
		}).Info("Group")
	}
}

// FetchEPG fetches and parses EPG data from multiple sources, merging with priority.
func (f *Fetcher) FetchEPG(ctx context.Context) error {
	m3uChannels, ok := f.store.GetM3U()
	if !ok {
		return fmt.Errorf("M3U data not available, cannot filter EPG")
	}

	results := make([]*epg.FilterResult, 0, len(f.epgURLs))

	for i, epgURL := range f.epgURLs {
		f.log.WithFields(logrus.Fields{
			"url":      epgURL,
			"priority": i + 1,
			"total":    len(f.epgURLs),
		}).Info("Fetching EPG source")

		data, err := f.fetch(ctx, epgURL)
		if err != nil {
			f.log.WithError(err).WithField("url", epgURL).Warn("Failed to fetch EPG source")

			continue
		}

		epgData, err := epg.Parse(data)
		if err != nil {
			f.log.WithError(err).WithField("url", epgURL).Warn("Failed to parse EPG source")

			continue
		}

		result := epg.FilterForMerge(f.log, epgData, m3uChannels)
		results = append(results, result)

		f.log.WithFields(logrus.Fields{
			"url":        epgURL,
			"channels":   len(result.ChannelMap),
			"programmes": len(result.EPG.Programs),
		}).Info("Filtered EPG source")
	}

	if len(results) == 0 {
		return fmt.Errorf("all EPG sources failed")
	}

	// Merge all results with program-level deduplication.
	merged := epg.MergeEPGs(results)

	// Build final TV struct.
	finalEPG := &epg.TV{
		Channels: merged.Channels,
		Programs: merged.Programs,
	}

	// Add fake channels for unmatched M3U channels.
	finalEPG = epg.AddFakeChannels(f.log, finalEPG, m3uChannels, merged.ChannelMap)

	f.store.SetEPG(finalEPG, merged.ChannelMap)

	f.log.WithFields(logrus.Fields{
		"sources":    len(results),
		"channels":   len(finalEPG.Channels),
		"programmes": len(finalEPG.Programs),
	}).Info("Merged EPG data from all sources")

	return nil
}

func (f *Fetcher) fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Accept gzip encoding
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var reader io.Reader = resp.Body

	// Handle gzip encoding
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gzReader, gzErr := gzip.NewReader(resp.Body)
		if gzErr != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", gzErr)
		}
		defer gzReader.Close()

		reader = gzReader
	}

	limitedReader := io.LimitReader(reader, maxBodySize)

	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	f.log.WithField("size", len(data)).Debug("Fetched data")

	return data, nil
}
