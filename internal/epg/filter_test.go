package epg

import (
	"io"
	"testing"

	"github.com/savid/iptv/internal/m3u"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func newTestLogger() logrus.FieldLogger {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	return logger
}

func TestFilter_MatchingChannels(t *testing.T) {
	log := newTestLogger()

	epgData := &TV{
		Channels: []Channel{
			{ID: "espn.us", DisplayName: "ESPN", Icon: Icon{Src: "http://logo.example.com/espn.png"}},
			{ID: "hbo.us", DisplayName: "HBO", Icon: Icon{Src: "http://logo.example.com/hbo.png"}},
			{ID: "cnn.us", DisplayName: "CNN", Icon: Icon{Src: "http://logo.example.com/cnn.png"}},
		},
		Programs: []Programme{
			{Channel: "espn.us", Start: "20260104120000 +0000", Stop: "20260104130000 +0000", Title: "SportsCenter"},
			{Channel: "hbo.us", Start: "20260104120000 +0000", Stop: "20260104140000 +0000", Title: "Movie"},
			{Channel: "cnn.us", Start: "20260104120000 +0000", Stop: "20260104130000 +0000", Title: "News"},
		},
	}

	m3uChannels := []m3u.Channel{
		{Name: "ESPN", URL: "http://stream.example.com/1"},
		{Name: "HBO", URL: "http://stream.example.com/2"},
	}

	filtered, channelMap := Filter(log, epgData, m3uChannels)

	require.Len(t, filtered.Channels, 2)
	require.Len(t, filtered.Programs, 2)

	require.Equal(t, "ESPN", channelMap["espn.us"])
	require.Equal(t, "HBO", channelMap["hbo.us"])

	_, hasCNN := channelMap["cnn.us"]

	require.False(t, hasCNN)
}

func TestFilter_NoMatchingChannels(t *testing.T) {
	log := newTestLogger()

	epgData := &TV{
		Channels: []Channel{
			{ID: "unknown.channel", DisplayName: "Unknown Channel"},
		},
		Programs: []Programme{
			{Channel: "unknown.channel", Start: "20260104120000 +0000", Stop: "20260104130000 +0000", Title: "Show"},
		},
	}

	m3uChannels := []m3u.Channel{
		{Name: "ESPN", URL: "http://stream.example.com/1", TVGLogo: "http://logo.example.com/espn.png"},
		{Name: "HBO", URL: "http://stream.example.com/2", TVGLogo: "http://logo.example.com/hbo.png"},
	}

	filtered, channelMap := Filter(log, epgData, m3uChannels)

	require.Len(t, filtered.Channels, 2)
	require.Len(t, filtered.Programs, 2)

	for _, ch := range filtered.Channels {
		require.Contains(t, []string{"ESPN", "HBO"}, ch.DisplayName)
	}

	require.Len(t, channelMap, 2)
}

func TestFilter_PartialMatch(t *testing.T) {
	log := newTestLogger()

	epgData := &TV{
		Channels: []Channel{
			{ID: "espn.us", DisplayName: "ESPN"},
		},
		Programs: []Programme{
			{Channel: "espn.us", Start: "20260104120000 +0000", Stop: "20260104130000 +0000", Title: "SportsCenter"},
		},
	}

	m3uChannels := []m3u.Channel{
		{Name: "ESPN", URL: "http://stream.example.com/1"},
		{Name: "HBO", URL: "http://stream.example.com/2", TVGLogo: "http://logo.example.com/hbo.png"},
	}

	filtered, channelMap := Filter(log, epgData, m3uChannels)

	require.Len(t, filtered.Channels, 2)
	require.Len(t, filtered.Programs, 2)

	hasESPN := false
	hasHBO := false

	for _, ch := range filtered.Channels {
		if ch.DisplayName == "ESPN" {
			hasESPN = true

			require.Equal(t, "espn.us", ch.ID)
		}

		if ch.DisplayName == "HBO" {
			hasHBO = true
		}
	}

	require.True(t, hasESPN)
	require.True(t, hasHBO)

	require.Len(t, channelMap, 2)
}

func TestFilter_DuplicateEPGChannels(t *testing.T) {
	log := newTestLogger()

	epgData := &TV{
		Channels: []Channel{
			{ID: "espn.1", DisplayName: "ESPN"},
			{ID: "espn.2", DisplayName: "ESPN"},
		},
		Programs: []Programme{
			{Channel: "espn.1", Start: "20260104120000 +0000", Stop: "20260104130000 +0000", Title: "Show 1"},
			{Channel: "espn.2", Start: "20260104120000 +0000", Stop: "20260104130000 +0000", Title: "Show 2"},
		},
	}

	m3uChannels := []m3u.Channel{
		{Name: "ESPN", URL: "http://stream.example.com/1"},
	}

	filtered, channelMap := Filter(log, epgData, m3uChannels)

	require.Len(t, filtered.Channels, 1)
	require.Equal(t, "ESPN", filtered.Channels[0].DisplayName)
	require.Len(t, channelMap, 1)
}

func TestFilter_DuplicateChannelIDs(t *testing.T) {
	log := newTestLogger()

	epgData := &TV{
		Channels: []Channel{
			{ID: "same.id", DisplayName: "Channel A"},
			{ID: "same.id", DisplayName: "Channel B"},
		},
		Programs: []Programme{
			{Channel: "same.id", Start: "20260104120000 +0000", Stop: "20260104130000 +0000", Title: "Show"},
		},
	}

	m3uChannels := []m3u.Channel{
		{Name: "Channel A", URL: "http://stream.example.com/a"},
		{Name: "Channel B", URL: "http://stream.example.com/b"},
	}

	filtered, channelMap := Filter(log, epgData, m3uChannels)

	require.Len(t, filtered.Channels, 2)

	ids := make([]string, 0, len(filtered.Channels))

	for _, ch := range filtered.Channels {
		ids = append(ids, ch.ID)
	}

	require.Contains(t, ids, "same.id")
	require.Contains(t, ids, "same.id-2")

	require.Len(t, channelMap, 2)
}

func TestFilter_EmptyChannelID(t *testing.T) {
	log := newTestLogger()

	epgData := &TV{
		Channels: []Channel{
			{ID: "", DisplayName: "ESPN"},
		},
		Programs: []Programme{},
	}

	m3uChannels := []m3u.Channel{
		{Name: "ESPN", URL: "http://stream.example.com/1"},
	}

	filtered, channelMap := Filter(log, epgData, m3uChannels)

	require.Len(t, filtered.Channels, 1)
	require.NotEmpty(t, filtered.Channels[0].ID)
	require.Len(t, channelMap, 1)
}

func TestFilter_ProgrammeFiltering(t *testing.T) {
	log := newTestLogger()

	epgData := &TV{
		Channels: []Channel{
			{ID: "espn.us", DisplayName: "ESPN"},
			{ID: "cnn.us", DisplayName: "CNN"},
		},
		Programs: []Programme{
			{Channel: "espn.us", Start: "20260104120000 +0000", Stop: "20260104130000 +0000", Title: "ESPN Show"},
			{Channel: "cnn.us", Start: "20260104120000 +0000", Stop: "20260104130000 +0000", Title: "CNN Show"},
			{Channel: "unknown", Start: "20260104120000 +0000", Stop: "20260104130000 +0000", Title: "Unknown Show"},
		},
	}

	m3uChannels := []m3u.Channel{
		{Name: "ESPN", URL: "http://stream.example.com/1"},
	}

	filtered, _ := Filter(log, epgData, m3uChannels)

	espnPrograms := 0

	for _, prog := range filtered.Programs {
		if prog.Title == "ESPN Show" {
			espnPrograms++
		}

		require.NotEqual(t, "Unknown Show", prog.Title)
		require.NotEqual(t, "CNN Show", prog.Title)
	}

	require.Equal(t, 1, espnPrograms)
}

func TestFilter_ProgrammeDuplication(t *testing.T) {
	log := newTestLogger()

	epgData := &TV{
		Channels: []Channel{
			{ID: "same.id", DisplayName: "Channel A"},
			{ID: "same.id", DisplayName: "Channel B"},
		},
		Programs: []Programme{
			{Channel: "same.id", Start: "20260104120000 +0000", Stop: "20260104130000 +0000", Title: "Shared Show"},
		},
	}

	m3uChannels := []m3u.Channel{
		{Name: "Channel A", URL: "http://stream.example.com/a"},
		{Name: "Channel B", URL: "http://stream.example.com/b"},
	}

	filtered, _ := Filter(log, epgData, m3uChannels)

	sharedShowCount := 0

	for _, prog := range filtered.Programs {
		if prog.Title == "Shared Show" {
			sharedShowCount++
		}
	}

	require.Equal(t, 2, sharedShowCount)
}

func TestFilter_GenerateFakeChannels(t *testing.T) {
	log := newTestLogger()

	epgData := &TV{
		Channels: []Channel{},
		Programs: []Programme{},
	}

	m3uChannels := []m3u.Channel{
		{Name: "New Channel", URL: "http://stream.example.com/1", TVGLogo: "http://logo.example.com/new.png"},
	}

	filtered, channelMap := Filter(log, epgData, m3uChannels)

	require.Len(t, filtered.Channels, 1)
	require.Equal(t, "New Channel", filtered.Channels[0].DisplayName)
	require.Equal(t, "http://logo.example.com/new.png", filtered.Channels[0].Icon.Src)
	require.NotEmpty(t, filtered.Channels[0].ID)

	require.Len(t, channelMap, 1)
}

func TestFilter_GenerateFakeProgrammes(t *testing.T) {
	log := newTestLogger()

	epgData := &TV{
		Channels: []Channel{
			{ID: "espn.us", DisplayName: "ESPN"},
		},
		Programs: []Programme{},
	}

	m3uChannels := []m3u.Channel{
		{Name: "ESPN", URL: "http://stream.example.com/1"},
	}

	filtered, _ := Filter(log, epgData, m3uChannels)

	require.Len(t, filtered.Programs, 1)
	require.Equal(t, "ESPN", filtered.Programs[0].Title)
	require.Equal(t, "No programme information available", filtered.Programs[0].Description)
}

func TestFilter_EmptyM3UChannels(t *testing.T) {
	log := newTestLogger()

	epgData := &TV{
		Channels: []Channel{
			{ID: "espn.us", DisplayName: "ESPN"},
		},
		Programs: []Programme{
			{Channel: "espn.us", Start: "20260104120000 +0000", Stop: "20260104130000 +0000", Title: "Show"},
		},
	}

	m3uChannels := []m3u.Channel{}

	filtered, channelMap := Filter(log, epgData, m3uChannels)

	require.Empty(t, filtered.Channels)
	require.Empty(t, filtered.Programs)
	require.Empty(t, channelMap)
}

func TestFilter_EmptyChannelNames(t *testing.T) {
	log := newTestLogger()

	epgData := &TV{
		Channels: []Channel{},
		Programs: []Programme{},
	}

	m3uChannels := []m3u.Channel{
		{Name: "", URL: "http://stream.example.com/1"},
		{Name: "Valid", URL: "http://stream.example.com/2"},
	}

	filtered, channelMap := Filter(log, epgData, m3uChannels)

	require.Len(t, filtered.Channels, 1)
	require.Equal(t, "Valid", filtered.Channels[0].DisplayName)
	require.Len(t, channelMap, 1)
}

func TestBuildChannelNameMap(t *testing.T) {
	tests := []struct {
		name     string
		channels []m3u.Channel
		expected map[string]bool
	}{
		{
			name:     "empty channels",
			channels: []m3u.Channel{},
			expected: map[string]bool{},
		},
		{
			name: "multiple channels",
			channels: []m3u.Channel{
				{Name: "ESPN"},
				{Name: "HBO"},
				{Name: "CNN"},
			},
			expected: map[string]bool{"ESPN": true, "HBO": true, "CNN": true},
		},
		{
			name: "channels with empty names",
			channels: []m3u.Channel{
				{Name: "ESPN"},
				{Name: ""},
				{Name: "HBO"},
			},
			expected: map[string]bool{"ESPN": true, "HBO": true},
		},
		{
			name: "duplicate channel names",
			channels: []m3u.Channel{
				{Name: "ESPN"},
				{Name: "ESPN"},
			},
			expected: map[string]bool{"ESPN": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildChannelNameMap(tt.channels)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildTVGIDMap(t *testing.T) {
	tests := []struct {
		name     string
		channels []m3u.Channel
		expected map[string]string
	}{
		{
			name:     "empty channels",
			channels: []m3u.Channel{},
			expected: map[string]string{},
		},
		{
			name: "channels with tvg-id",
			channels: []m3u.Channel{
				{Name: "ESPN", TVGID: "espn.us"},
				{Name: "CNN", TVGID: "cnn.us"},
			},
			expected: map[string]string{"espn.us": "ESPN", "cnn.us": "CNN"},
		},
		{
			name: "channels without tvg-id",
			channels: []m3u.Channel{
				{Name: "ESPN", TVGID: ""},
				{Name: "CNN"},
			},
			expected: map[string]string{},
		},
		{
			name: "mixed channels",
			channels: []m3u.Channel{
				{Name: "ESPN", TVGID: "espn.us"},
				{Name: "HBO", TVGID: ""},
				{Name: "CNN", TVGID: "cnn.us"},
			},
			expected: map[string]string{"espn.us": "ESPN", "cnn.us": "CNN"},
		},
		{
			name: "channel with empty name ignored",
			channels: []m3u.Channel{
				{Name: "", TVGID: "orphan.id"},
				{Name: "ESPN", TVGID: "espn.us"},
			},
			expected: map[string]string{"espn.us": "ESPN"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildTVGIDMap(tt.channels)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchChannelsByTVGID(t *testing.T) {
	log := newTestLogger()

	m3uChannels := []m3u.Channel{
		{Name: "US: ESPN", TVGID: "espn.us"},
		{Name: "US: CNN", TVGID: "cnn.us"},
		{Name: "Local News", TVGID: ""}, // No tvg-id, will match by name
	}

	epgChannels := []Channel{
		{ID: "espn.us", DisplayName: "ESPN"},
		{ID: "cnn.us", DisplayName: "CNN"},
		{ID: "local.news", DisplayName: "Local News"},
	}

	channelNameMap := buildChannelNameMap(m3uChannels)
	tvgIDMap := buildTVGIDMap(m3uChannels)
	normalizedNameMap := buildNormalizedNameMap(m3uChannels)

	matched, idMap := matchChannels(log, epgChannels, channelNameMap, tvgIDMap, normalizedNameMap)

	require.Len(t, matched, 3)
	// Matched by tvg-id
	require.Equal(t, "US: ESPN", idMap["espn.us"])
	require.Equal(t, "US: CNN", idMap["cnn.us"])
	// Matched by display name
	require.Equal(t, "Local News", idMap["local.news"])
}

func TestMatchChannelsTVGIDPriority(t *testing.T) {
	log := newTestLogger()

	// M3U channel has tvg-id that matches EPG, but different display name
	m3uChannels := []m3u.Channel{
		{Name: "US: ESPN HD", TVGID: "espn.us"},
	}

	epgChannels := []Channel{
		{ID: "espn.us", DisplayName: "ESPN"}, // Different display name
	}

	channelNameMap := buildChannelNameMap(m3uChannels)
	tvgIDMap := buildTVGIDMap(m3uChannels)
	normalizedNameMap := buildNormalizedNameMap(m3uChannels)

	matched, idMap := matchChannels(log, epgChannels, channelNameMap, tvgIDMap, normalizedNameMap)

	require.Len(t, matched, 1)
	// Should match by tvg-id, returning M3U channel name
	require.Equal(t, "US: ESPN HD", idMap["espn.us"])
}

func TestGenerateChannelID(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
	}{
		{name: "simple name", displayName: "ESPN"},
		{name: "name with spaces", displayName: "FOX Sports 1"},
		{name: "unicode name", displayName: "Télé Zürich"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := generateChannelID(tt.displayName)
			require.NotEmpty(t, id)
			require.Len(t, id, 32)

			id2 := generateChannelID(tt.displayName)
			require.Equal(t, id, id2)
		})
	}

	id1 := generateChannelID("ESPN")
	id2 := generateChannelID("HBO")

	require.NotEqual(t, id1, id2)
}

func TestIsNumericSuffix(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"1", true},
		{"2", true},
		{"123", true},
		{"0", true},
		{"", false},
		{"a", false},
		{"1a", false},
		{"a1", false},
		{"-1", true}, // strconv.Atoi parses negative numbers
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isNumericSuffix(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFilter_CategoryPopulation(t *testing.T) {
	log := newTestLogger()

	epgData := &TV{
		Channels: []Channel{
			{ID: "espn.us", DisplayName: "ESPN"},
			{ID: "hbo.us", DisplayName: "HBO"},
		},
		Programs: []Programme{
			{Channel: "espn.us", Start: "20260104120000 +0000", Stop: "20260104130000 +0000", Title: "SportsCenter"},
			{Channel: "hbo.us", Start: "20260104120000 +0000", Stop: "20260104140000 +0000", Title: "Movie"},
		},
	}

	m3uChannels := []m3u.Channel{
		{Name: "ESPN", URL: "http://stream.example.com/1", Group: "US Sports"},
		{Name: "HBO", URL: "http://stream.example.com/2", Group: "US Movies"},
	}

	filtered, _ := Filter(log, epgData, m3uChannels)

	require.Len(t, filtered.Programs, 2)

	categoryMap := make(map[string]string)

	for _, prog := range filtered.Programs {
		categoryMap[prog.Title] = prog.Category
	}

	require.Equal(t, "US Sports", categoryMap["SportsCenter"])
	require.Equal(t, "US Movies", categoryMap["Movie"])
}

func TestFilter_CategoryPopulationForFakeChannels(t *testing.T) {
	log := newTestLogger()

	epgData := &TV{
		Channels: []Channel{},
		Programs: []Programme{},
	}

	m3uChannels := []m3u.Channel{
		{Name: "New Sports Channel", URL: "http://stream.example.com/1", Group: "Sports"},
		{Name: "New Movie Channel", URL: "http://stream.example.com/2", Group: "Movies"},
	}

	filtered, _ := Filter(log, epgData, m3uChannels)

	require.Len(t, filtered.Programs, 2)

	categoryMap := make(map[string]string)

	for _, prog := range filtered.Programs {
		categoryMap[prog.Title] = prog.Category
	}

	require.Equal(t, "Sports", categoryMap["New Sports Channel"])
	require.Equal(t, "Movies", categoryMap["New Movie Channel"])
}

func TestFilter_CategoryPopulationForFakeProgrammes(t *testing.T) {
	log := newTestLogger()

	epgData := &TV{
		Channels: []Channel{
			{ID: "espn.us", DisplayName: "ESPN"},
		},
		Programs: []Programme{},
	}

	m3uChannels := []m3u.Channel{
		{Name: "ESPN", URL: "http://stream.example.com/1", Group: "US Sports"},
	}

	filtered, _ := Filter(log, epgData, m3uChannels)

	require.Len(t, filtered.Programs, 1)
	require.Equal(t, "ESPN", filtered.Programs[0].Title)
	require.Equal(t, "US Sports", filtered.Programs[0].Category)
}

func TestBuildCategoryMap(t *testing.T) {
	tests := []struct {
		name     string
		channels []m3u.Channel
		expected map[string]string
	}{
		{
			name:     "empty channels",
			channels: []m3u.Channel{},
			expected: map[string]string{},
		},
		{
			name: "channels with groups",
			channels: []m3u.Channel{
				{Name: "ESPN", Group: "Sports"},
				{Name: "HBO", Group: "Movies"},
			},
			expected: map[string]string{"ESPN": "Sports", "HBO": "Movies"},
		},
		{
			name: "channels without groups",
			channels: []m3u.Channel{
				{Name: "ESPN", Group: "Sports"},
				{Name: "HBO", Group: ""},
			},
			expected: map[string]string{"ESPN": "Sports"},
		},
		{
			name: "empty names ignored",
			channels: []m3u.Channel{
				{Name: "", Group: "Sports"},
				{Name: "HBO", Group: "Movies"},
			},
			expected: map[string]string{"HBO": "Movies"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildCategoryMap(tt.channels)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeChannelName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple name",
			input:    "ESPN",
			expected: "espn",
		},
		{
			name:     "US prefix with colon",
			input:    "US: ESPN",
			expected: "espn",
		},
		{
			name:     "USA prefix with spaces",
			input:    "USA  ESPN",
			expected: "espn",
		},
		{
			name:     "AU prefix",
			input:    "AU: Fox Sports 501",
			expected: "fox sports 501",
		},
		{
			name:     "quality suffix HD",
			input:    "ESPN (HD)",
			expected: "espn",
		},
		{
			name:     "multiple suffixes",
			input:    "US ESPN 1 (HD) (S)",
			expected: "espn 1",
		},
		{
			name:     "Carib prefix",
			input:    "Carib ESPN (D)",
			expected: "espn",
		},
		{
			name:     "normalize whitespace",
			input:    "FOX   NEWS  ",
			expected: "fox news",
		},
		{
			name:     "EMEA suffix",
			input:    "PH TFC (EMEA)",
			expected: "tfc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeChannelName(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildNormalizedNameMap(t *testing.T) {
	tests := []struct {
		name     string
		channels []m3u.Channel
		expected map[string]m3uNormalizedInfo
	}{
		{
			name:     "empty channels",
			channels: []m3u.Channel{},
			expected: map[string]m3uNormalizedInfo{},
		},
		{
			name: "channels without tvg-id",
			channels: []m3u.Channel{
				{Name: "USA  ESPN", TVGID: ""},
				{Name: "USA  CNN", TVGID: ""},
			},
			expected: map[string]m3uNormalizedInfo{
				"espn": {originalName: "USA  ESPN", normalizedName: "espn", region: "us"},
				"cnn":  {originalName: "USA  CNN", normalizedName: "cnn", region: "us"},
			},
		},
		{
			name: "skip channels with tvg-id",
			channels: []m3u.Channel{
				{Name: "US: ESPN", TVGID: "espn.us"},
				{Name: "USA  ESPN", TVGID: ""},
			},
			expected: map[string]m3uNormalizedInfo{
				"espn": {originalName: "USA  ESPN", normalizedName: "espn", region: "us"},
			},
		},
		{
			name: "skip duplicate names that have tvg-id variant",
			channels: []m3u.Channel{
				{Name: "US: ESPN", TVGID: "espn.us"},
				{Name: "US: ESPN", TVGID: ""},
				{Name: "USA  ESPN", TVGID: ""},
			},
			expected: map[string]m3uNormalizedInfo{
				"espn": {originalName: "USA  ESPN", normalizedName: "espn", region: "us"},
			},
		},
		{
			name: "first occurrence wins for same normalized name",
			channels: []m3u.Channel{
				{Name: "USA  ESPN", TVGID: ""},
				{Name: "US: ESPN (HD)", TVGID: ""},
			},
			expected: map[string]m3uNormalizedInfo{
				"espn": {originalName: "USA  ESPN", normalizedName: "espn", region: "us"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildNormalizedNameMap(tt.channels)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchChannelsByNormalizedName(t *testing.T) {
	log := newTestLogger()

	m3uChannels := []m3u.Channel{
		{Name: "US: ESPN", TVGID: "espn.us"},
		{Name: "USA  CNN", TVGID: ""},  // No tvg-id, should match via normalized
		{Name: "Carib FOX", TVGID: ""}, // No tvg-id, should match via normalized
	}

	epgChannels := []Channel{
		{ID: "espn.us", DisplayName: "ESPN"},
		{ID: "", DisplayName: "ID CNN (D)"}, // Normalizes to "cnn"
		{ID: "", DisplayName: "UK: FOX"},    // Normalizes to "fox"
	}

	channelNameMap := buildChannelNameMap(m3uChannels)
	tvgIDMap := buildTVGIDMap(m3uChannels)
	normalizedNameMap := buildNormalizedNameMap(m3uChannels)

	matched, idMap := matchChannels(log, epgChannels, channelNameMap, tvgIDMap, normalizedNameMap)

	require.Len(t, matched, 3)
	// Matched by tvg-id
	require.Equal(t, "US: ESPN", idMap["espn.us"])
	// Matched by normalized name
	foundCNN := false
	foundFOX := false

	for id, name := range idMap {
		if name == "USA  CNN" {
			foundCNN = true

			require.NotEqual(t, "espn.us", id)
		}

		if name == "Carib FOX" {
			foundFOX = true
		}
	}

	require.True(t, foundCNN, "USA  CNN should be matched via normalized name")
	require.True(t, foundFOX, "Carib FOX should be matched via normalized name")
}
