package epg

import (
	"crypto/md5" //nolint:gosec // MD5 is used for ID generation, not security
	"fmt"
	"strconv"
	"strings"

	"github.com/savid/iptv/internal/m3u"
	"github.com/sirupsen/logrus"
)

// Common country/region prefixes to strip for normalized matching.
// Order matters: longer/more specific prefixes should come first.
var countryPrefixes = []string{
	// Double-space variants first (more specific)
	"USA  ", "World  ", "AUS  ",
	// Colon variants
	"US:", "AU:", "AUS:", "UK:", "PH:", "BR:", "CA:", "NZ:", "MX:", "ID:",
	// Space variants
	"USA ", "UK ", "PH ", "BR ", "ID ", "MY ", "MX ", "AUS ",
	// Multi-word prefixes
	"Carib ", "World ", "Latin ", "US ",
}

// regionPrefixes maps normalized region codes for region-aware matching.
var regionPrefixes = map[string]string{
	"US:": "us", "USA ": "us", "USA  ": "us", "US ": "us",
	"AU:": "au", "AUS:": "au", "AUS ": "au", "AUS  ": "au",
	"UK:": "uk", "UK ": "uk",
	"PH:": "ph", "PH ": "ph",
	"BR:": "br", "BR ": "br",
	"CA:": "ca",
	"NZ:": "nz",
	"MX:": "mx", "MX ": "mx",
	"ID:": "id", "ID ": "id",
	"MY ":    "my",
	"Carib ": "carib",
	"World ": "world", "World  ": "world",
	"Latin ": "latin",
}

// Common quality/variant suffixes to strip for normalized matching.
var qualitySuffixes = []string{
	"(HD)", "(FHD)", "(SD)", "(4K)", "(UHD)",
	"(S)", "(A)", "(H)", "(D)", "(C)", "(P)", "(FL)", "(F)", "(E)", "(R)",
	"(North America)", "(EMEA)", "(PRIME)", "(TUBI)",
	" FHD", " HD",
}

// extractRegion returns the normalized region code from a channel name, or empty string if none.
func extractRegion(name string) string {
	upperName := strings.ToUpper(name)

	for prefix, region := range regionPrefixes {
		if strings.HasPrefix(upperName, strings.ToUpper(prefix)) {
			return region
		}
	}

	return ""
}

// normalizeChannelName strips country prefixes, quality suffixes, and normalizes whitespace.
func normalizeChannelName(name string) string {
	normalized := name

	// Strip country prefixes (case-insensitive).
	upperName := strings.ToUpper(normalized)
	for _, prefix := range countryPrefixes {
		if strings.HasPrefix(upperName, strings.ToUpper(prefix)) {
			normalized = normalized[len(prefix):]
			upperName = strings.ToUpper(normalized)
		}
	}

	// Strip quality suffixes.
	for _, suffix := range qualitySuffixes {
		upperName = strings.ToUpper(normalized)
		upperSuffix := strings.ToUpper(suffix)

		for strings.Contains(upperName, upperSuffix) {
			idx := strings.Index(upperName, upperSuffix)
			if idx >= 0 {
				normalized = normalized[:idx] + normalized[idx+len(suffix):]
				upperName = strings.ToUpper(normalized)
			}
		}
	}

	// Normalize whitespace: collapse multiple spaces, trim.
	normalized = strings.Join(strings.Fields(normalized), " ")
	normalized = strings.TrimSpace(normalized)

	// Convert to lowercase for comparison.
	return strings.ToLower(normalized)
}

// m3uNormalizedInfo holds normalized name and region for an M3U channel.
type m3uNormalizedInfo struct {
	originalName   string
	normalizedName string
	region         string
}

// buildNormalizedNameMap creates a map from normalized M3U channel names to channel info.
// Only includes channels WITHOUT tvg-id, since channels with tvg-id should match via tvg-id.
// Also skips channels whose name has a tvg-id variant (which will match via tvg-id instead).
func buildNormalizedNameMap(m3uChannels []m3u.Channel) map[string]m3uNormalizedInfo {
	// First, find all channel names that have a tvg-id variant.
	namesWithTVGID := make(map[string]bool, len(m3uChannels))

	for _, channel := range m3uChannels {
		if channel.TVGID != "" && channel.Name != "" {
			namesWithTVGID[channel.Name] = true
		}
	}

	normalizedMap := make(map[string]m3uNormalizedInfo, len(m3uChannels))

	for _, channel := range m3uChannels {
		// Skip channels with tvg-id - they should match via tvg-id, not normalized name.
		if channel.TVGID != "" {
			continue
		}

		// Skip channels whose name has a tvg-id variant (duplicate entry that will match via tvg-id).
		if namesWithTVGID[channel.Name] {
			continue
		}

		if channel.Name != "" {
			normalized := normalizeChannelName(channel.Name)
			region := extractRegion(channel.Name)

			// Only store first occurrence (prefer earlier channels).
			if _, exists := normalizedMap[normalized]; !exists {
				normalizedMap[normalized] = m3uNormalizedInfo{
					originalName:   channel.Name,
					normalizedName: normalized,
					region:         region,
				}
			}
		}
	}

	return normalizedMap
}

// FilterForMerge filters EPG data without generating fake channels.
// Used when merging multiple EPG sources - fake data is added after merging.
func FilterForMerge(log logrus.FieldLogger, epgData *TV, m3uChannels []m3u.Channel) *FilterResult {
	channelNameMap := buildChannelNameMap(m3uChannels)
	tvgIDMap := buildTVGIDMap(m3uChannels)
	normalizedNameMap := buildNormalizedNameMap(m3uChannels)

	categoryMap := buildCategoryMap(m3uChannels)
	matchedChannels, channelIDMap := matchChannels(log, epgData.Channels, channelNameMap, tvgIDMap, normalizedNameMap)

	// Track original IDs for duplicated channels.
	originalIDMap := make(map[string][]string, len(channelIDMap))

	for channelID := range channelIDMap {
		if idx := strings.LastIndex(channelID, "-"); idx > 0 {
			if suffix := channelID[idx+1:]; isNumericSuffix(suffix) {
				originalID := channelID[:idx]
				originalIDMap[originalID] = append(originalIDMap[originalID], channelID)
			}
		}
	}

	filteredPrograms := make([]Programme, 0, len(epgData.Programs))

	for _, program := range epgData.Programs {
		if displayName, exists := channelIDMap[program.Channel]; exists {
			programWithCategory := program
			if category, ok := categoryMap[displayName]; ok {
				programWithCategory.Category = category
			}

			filteredPrograms = append(filteredPrograms, programWithCategory)
		}

		if suffixedIDs, exists := originalIDMap[program.Channel]; exists {
			for _, suffixedID := range suffixedIDs {
				duplicatedProgram := program
				duplicatedProgram.Channel = suffixedID

				if displayName, ok := channelIDMap[suffixedID]; ok {
					if category, catOK := categoryMap[displayName]; catOK {
						duplicatedProgram.Category = category
					}
				}

				filteredPrograms = append(filteredPrograms, duplicatedProgram)
			}
		}
	}

	return &FilterResult{
		EPG: &TV{
			XMLName:  epgData.XMLName,
			Channels: matchedChannels,
			Programs: filteredPrograms,
		},
		ChannelMap: channelIDMap,
	}
}

// Filter filters EPG data to only include channels and programs that match the M3U playlist.
// Returns the filtered EPG and a map of channel IDs to display names.
func Filter(log logrus.FieldLogger, epgData *TV, m3uChannels []m3u.Channel) (*TV, map[string]string) {
	channelNameMap := buildChannelNameMap(m3uChannels)
	tvgIDMap := buildTVGIDMap(m3uChannels)
	normalizedNameMap := buildNormalizedNameMap(m3uChannels)

	categoryMap := buildCategoryMap(m3uChannels)
	matchedChannels, channelIDMap := matchChannels(log, epgData.Channels, channelNameMap, tvgIDMap, normalizedNameMap)

	channelsWithPrograms := make(map[string]bool, len(matchedChannels))

	// Track original IDs for duplicated channels.
	originalIDMap := make(map[string][]string, len(channelIDMap))

	for channelID := range channelIDMap {
		if idx := strings.LastIndex(channelID, "-"); idx > 0 {
			if suffix := channelID[idx+1:]; isNumericSuffix(suffix) {
				originalID := channelID[:idx]
				originalIDMap[originalID] = append(originalIDMap[originalID], channelID)
			}
		}
	}

	filteredPrograms := make([]Programme, 0, len(epgData.Programs))

	for _, program := range epgData.Programs {
		if displayName, exists := channelIDMap[program.Channel]; exists {
			programWithCategory := program
			if category, ok := categoryMap[displayName]; ok {
				programWithCategory.Category = category
			}

			filteredPrograms = append(filteredPrograms, programWithCategory)
			channelsWithPrograms[program.Channel] = true
		}

		if suffixedIDs, exists := originalIDMap[program.Channel]; exists {
			for _, suffixedID := range suffixedIDs {
				duplicatedProgram := program
				duplicatedProgram.Channel = suffixedID

				if displayName, ok := channelIDMap[suffixedID]; ok {
					if category, catOK := categoryMap[displayName]; catOK {
						duplicatedProgram.Category = category
					}
				}

				filteredPrograms = append(filteredPrograms, duplicatedProgram)
				channelsWithPrograms[suffixedID] = true
			}
		}
	}

	// Generate EPG channel entries for unmatched M3U channels (no guide data, but channel exists).
	fakeChannels := generateFakeEPGData(log, m3uChannels, channelIDMap)
	matchedChannels = append(matchedChannels, fakeChannels...)

	for _, fakeChannel := range fakeChannels {
		channelIDMap[fakeChannel.ID] = fakeChannel.DisplayName
	}

	// Generate fake programs for channels without program data.
	fakePrograms := generateFakePrograms(matchedChannels, channelsWithPrograms, categoryMap, channelIDMap)
	filteredPrograms = append(filteredPrograms, fakePrograms...)

	return &TV{
		XMLName:  epgData.XMLName,
		Channels: matchedChannels,
		Programs: filteredPrograms,
	}, channelIDMap
}

// buildChannelNameMap creates a map of M3U channel names for display-name matching.
func buildChannelNameMap(m3uChannels []m3u.Channel) map[string]bool {
	channelMap := make(map[string]bool, len(m3uChannels))

	for _, channel := range m3uChannels {
		if channel.Name != "" {
			channelMap[channel.Name] = true
		}
	}

	return channelMap
}

// buildTVGIDMap creates a map from tvg-id to M3U channel name for ID-based matching.
func buildTVGIDMap(m3uChannels []m3u.Channel) map[string]string {
	tvgIDMap := make(map[string]string, len(m3uChannels))

	for _, channel := range m3uChannels {
		if channel.TVGID != "" && channel.Name != "" {
			tvgIDMap[channel.TVGID] = channel.Name
		}
	}

	return tvgIDMap
}

// buildCategoryMap creates a map from channel name to category (group-title from M3U).
func buildCategoryMap(m3uChannels []m3u.Channel) map[string]string {
	categoryMap := make(map[string]string, len(m3uChannels))

	for _, channel := range m3uChannels {
		if channel.Name != "" && channel.Group != "" {
			categoryMap[channel.Name] = channel.Group
		}
	}

	return categoryMap
}

// matcherState holds shared state during channel matching.
type matcherState struct {
	log               logrus.FieldLogger
	epgChannels       []Channel
	matchedChannels   []Channel
	channelIDMap      map[string]string
	matchedM3U        map[string]bool
	matchedEPG        map[int]bool
	idUsageCount      map[string]int
	epgIDToCandidates map[string][]int
}

func newMatcherState(log logrus.FieldLogger, epgChannels []Channel) *matcherState {
	state := &matcherState{
		log:               log,
		epgChannels:       epgChannels,
		matchedChannels:   make([]Channel, 0, len(epgChannels)),
		channelIDMap:      make(map[string]string, len(epgChannels)),
		matchedM3U:        make(map[string]bool, len(epgChannels)),
		matchedEPG:        make(map[int]bool, len(epgChannels)),
		idUsageCount:      make(map[string]int, len(epgChannels)),
		epgIDToCandidates: make(map[string][]int, len(epgChannels)),
	}

	for i, ch := range epgChannels {
		if ch.ID != "" {
			state.epgIDToCandidates[ch.ID] = append(state.epgIDToCandidates[ch.ID], i)
		}
	}

	return state
}

func (s *matcherState) addMatch(epgIdx int, m3uName string, logMsg string) {
	s.matchedEPG[epgIdx] = true
	s.matchedM3U[m3uName] = true

	epgCopy := s.epgChannels[epgIdx]
	if epgCopy.ID == "" {
		epgCopy.ID = generateChannelID(epgCopy.DisplayName)
		s.log.WithFields(logrus.Fields{
			"channel": epgCopy.DisplayName,
			"id":      epgCopy.ID,
		}).Debug("Generated ID for EPG channel with empty ID")
	}

	originalID := epgCopy.ID
	if count, exists := s.idUsageCount[originalID]; exists {
		epgCopy.ID = fmt.Sprintf("%s-%d", originalID, count+1)
		s.log.WithFields(logrus.Fields{
			"channel":    m3uName,
			"originalID": originalID,
			"newID":      epgCopy.ID,
		}).Debug("Appended suffix to duplicate channel ID")
	}

	s.idUsageCount[originalID]++
	s.matchedChannels = append(s.matchedChannels, epgCopy)
	s.channelIDMap[epgCopy.ID] = m3uName

	s.log.WithFields(logrus.Fields{
		"m3uChannel": m3uName,
		"epgID":      originalID,
	}).Debug(logMsg)
}

func (s *matcherState) matchByTVGID(tvgIDMap map[string]string) {
	for tvgID, m3uName := range tvgIDMap {
		if s.matchedM3U[m3uName] {
			continue
		}

		candidates := s.epgIDToCandidates[tvgID]
		if len(candidates) == 0 {
			continue
		}

		bestIdx := s.findBestTVGIDCandidate(candidates, m3uName)
		if bestIdx >= 0 {
			s.addMatch(bestIdx, m3uName, "Matched channel by tvg-id")
		}
	}
}

func (s *matcherState) findBestTVGIDCandidate(candidates []int, m3uName string) int {
	bestIdx := -1

	for _, idx := range candidates {
		if s.matchedEPG[idx] {
			continue
		}

		if s.epgChannels[idx].DisplayName == m3uName {
			return idx // Perfect match.
		}

		if bestIdx == -1 {
			bestIdx = idx
		}
	}

	return bestIdx
}

func (s *matcherState) matchByDisplayName(channelNameMap map[string]bool) {
	for i, epgChannel := range s.epgChannels {
		if s.matchedEPG[i] {
			continue
		}

		if !channelNameMap[epgChannel.DisplayName] || s.matchedM3U[epgChannel.DisplayName] {
			continue
		}

		s.addMatch(i, epgChannel.DisplayName, "Matched channel by display-name")
	}
}

func (s *matcherState) matchByNormalizedName(normalizedNameMap map[string]m3uNormalizedInfo) {
	for _, m3uInfo := range normalizedNameMap {
		if s.matchedM3U[m3uInfo.originalName] {
			continue
		}

		bestIdx := s.findBestNormalizedMatch(m3uInfo)
		if bestIdx >= 0 {
			s.log.WithFields(logrus.Fields{
				"m3uChannel":     m3uInfo.originalName,
				"epgDisplayName": s.epgChannels[bestIdx].DisplayName,
				"region":         m3uInfo.region,
			}).Debug("Matched channel by normalized name")

			s.addMatch(bestIdx, m3uInfo.originalName, "Matched channel by normalized name")
		}
	}
}

func (s *matcherState) findBestNormalizedMatch(m3uInfo m3uNormalizedInfo) int {
	bestIdx := -1
	bestScore := -1

	for i, epgChannel := range s.epgChannels {
		if s.matchedEPG[i] {
			continue
		}

		if normalizeChannelName(epgChannel.DisplayName) != m3uInfo.normalizedName {
			continue
		}

		score := scoreRegionMatch(m3uInfo.region, extractRegion(epgChannel.DisplayName))
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	return bestIdx
}

func scoreRegionMatch(m3uRegion, epgRegion string) int {
	if m3uRegion != "" && epgRegion == m3uRegion {
		return 2 // Same region = highest priority.
	}

	if epgRegion == "" {
		return 1 // No region in EPG = neutral.
	}

	return 0 // Different region = lowest.
}

func (s *matcherState) logUnmatched(channelNameMap map[string]bool) {
	var unmatched []string

	for name := range channelNameMap {
		if !s.matchedM3U[name] {
			unmatched = append(unmatched, name)
		}
	}

	if len(unmatched) > 0 {
		s.log.WithField("count", len(unmatched)).Warn("M3U channels have no EPG match")

		for _, ch := range unmatched {
			s.log.WithField("channel", ch).Debug("Unmatched M3U channel")
		}
	}

	s.log.WithField("matched", len(s.matchedChannels)).Info("Matched channels between M3U and EPG")
}

func matchChannels(
	log logrus.FieldLogger,
	epgChannels []Channel,
	channelNameMap map[string]bool,
	tvgIDMap map[string]string,
	normalizedNameMap map[string]m3uNormalizedInfo,
) ([]Channel, map[string]string) {
	state := newMatcherState(log, epgChannels)

	state.matchByTVGID(tvgIDMap)
	state.matchByDisplayName(channelNameMap)
	state.matchByNormalizedName(normalizedNameMap)
	state.logUnmatched(channelNameMap)

	return state.matchedChannels, state.channelIDMap
}

func generateFakeEPGData(
	log logrus.FieldLogger,
	m3uChannels []m3u.Channel,
	channelIDMap map[string]string,
) []Channel {
	// Build set of matched M3U names from channelIDMap values.
	matchedM3UNames := make(map[string]bool, len(channelIDMap))

	for _, m3uName := range channelIDMap {
		matchedM3UNames[m3uName] = true
	}

	fakeChannels := make([]Channel, 0, len(m3uChannels))

	for _, m3uChannel := range m3uChannels {
		if matchedM3UNames[m3uChannel.Name] {
			continue
		}

		if m3uChannel.Name == "" {
			continue
		}

		channelID := generateChannelID(m3uChannel.Name)

		fakeChannel := Channel{
			ID:          channelID,
			DisplayName: m3uChannel.Name,
			Icon: Icon{
				Src: m3uChannel.TVGLogo,
			},
		}
		fakeChannels = append(fakeChannels, fakeChannel)
	}

	if len(fakeChannels) > 0 {
		log.WithField("count", len(fakeChannels)).Info("Created EPG channel entries for unmatched M3U channels (no guide data)")
	}

	return fakeChannels
}

// generateFakePrograms creates placeholder program entries for channels without program data.
func generateFakePrograms(
	channels []Channel,
	channelsWithPrograms map[string]bool,
	categoryMap map[string]string,
	channelIDMap map[string]string,
) []Programme {
	fakePrograms := make([]Programme, 0)

	for _, ch := range channels {
		if channelsWithPrograms[ch.ID] {
			continue
		}

		displayName := ch.DisplayName
		if name, ok := channelIDMap[ch.ID]; ok {
			displayName = name
		}

		fakeProgram := Programme{
			Channel:     ch.ID,
			Start:       "20260101000000 +0000",
			Stop:        "20260101235959 +0000",
			Title:       displayName,
			Description: "No programme information available",
		}

		if category, ok := categoryMap[displayName]; ok {
			fakeProgram.Category = category
		}

		fakePrograms = append(fakePrograms, fakeProgram)
	}

	return fakePrograms
}

func generateChannelID(displayName string) string {
	hash := md5.Sum([]byte(displayName)) //nolint:gosec // MD5 is fine for ID generation

	return fmt.Sprintf("%x", hash)
}

func isNumericSuffix(s string) bool {
	if s == "" {
		return false
	}

	_, err := strconv.Atoi(s)

	return err == nil
}
