// Package data provides data storage and fetching for M3U playlists and EPG data.
package data

import (
	"sort"
	"sync"
	"time"

	"github.com/savid/iptv/internal/epg"
	"github.com/savid/iptv/internal/m3u"
)

// Store provides thread-safe storage for M3U and EPG data.
type Store struct {
	mu sync.RWMutex

	m3uChannels []m3u.Channel
	epgData     *epg.TV
	channelMap  map[string]string
	lastSync    time.Time
}

// NewStore creates a new data store.
func NewStore() *Store {
	return &Store{
		channelMap: make(map[string]string),
	}
}

// SetM3U updates the M3U channels.
func (s *Store) SetM3U(channels []m3u.Channel) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.m3uChannels = channels
	s.lastSync = time.Now()
}

// GetM3U returns the M3U channels.
func (s *Store) GetM3U() ([]m3u.Channel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.m3uChannels == nil {
		return nil, false
	}

	return s.m3uChannels, true
}

// SetEPG updates the EPG data.
func (s *Store) SetEPG(data *epg.TV, channelMap map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.epgData = data
	s.channelMap = channelMap
	s.lastSync = time.Now()
}

// GetEPG returns the EPG data.
func (s *Store) GetEPG() (*epg.TV, map[string]string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.epgData == nil {
		return nil, nil, false
	}

	return s.epgData, s.channelMap, true
}

// LastSync returns the last sync time.
func (s *Store) LastSync() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.lastSync
}

// HasData returns true if both M3U and EPG data are available.
func (s *Store) HasData() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.m3uChannels != nil && s.epgData != nil
}

// GetGroups returns all unique group-titles from M3U channels, sorted alphabetically.
func (s *Store) GetGroups() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := make(map[string]bool)
	groups := make([]string, 0)

	for _, ch := range s.m3uChannels {
		if ch.Group != "" && !seen[ch.Group] {
			seen[ch.Group] = true
			groups = append(groups, ch.Group)
		}
	}

	sort.Strings(groups)

	return groups
}

// GetChannelsByGroup returns channels matching a specific group.
// Empty group returns all channels.
func (s *Store) GetChannelsByGroup(group string) ([]m3u.Channel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.m3uChannels == nil {
		return nil, false
	}

	if group == "" {
		return s.m3uChannels, true
	}

	filtered := make([]m3u.Channel, 0)

	for _, ch := range s.m3uChannels {
		if ch.Group == group {
			filtered = append(filtered, ch)
		}
	}

	return filtered, true
}
