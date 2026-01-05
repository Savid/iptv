package epg

import (
	"github.com/savid/iptv/internal/m3u"
	"github.com/sirupsen/logrus"
)

// FilterResult holds the result of filtering one EPG source.
type FilterResult struct {
	EPG        *TV
	ChannelMap map[string]string // EPG ID → M3U name
}

// MergeResult holds the merged result from multiple EPG sources.
type MergeResult struct {
	Channels   []Channel
	Programs   []Programme
	ChannelMap map[string]string // EPG ID → M3U name
}

// MergeEPGs merges multiple filtered EPG results with program-level deduplication.
// Priority: earlier EPGs in the slice have higher priority for channel metadata.
// Programs from all EPGs are merged, with duplicates (same start time) skipped.
func MergeEPGs(results []*FilterResult) *MergeResult {
	merged := &MergeResult{
		Channels:   make([]Channel, 0, 100),
		Programs:   make([]Programme, 0, 1000),
		ChannelMap: make(map[string]string, 100),
	}

	if len(results) == 0 {
		return merged
	}

	// Track M3U name → primary EPG ID (first EPG to match owns the channel).
	m3uToEPGID := make(map[string]string, 100)

	// Track programs per channel for deduplication.
	channelPrograms := make(map[string][]Programme, 100)

	for _, r := range results {
		if r == nil || r.EPG == nil {
			continue
		}

		for epgID, m3uName := range r.ChannelMap {
			// First EPG to match a channel "owns" its metadata.
			if _, exists := m3uToEPGID[m3uName]; !exists {
				m3uToEPGID[m3uName] = epgID
				merged.ChannelMap[epgID] = m3uName

				// Add the channel entry with M3U name as display-name.
				// This ensures Plex can match the HDHomeRun GuideName to EPG.
				for _, ch := range r.EPG.Channels {
					if ch.ID == epgID {
						ch.DisplayName = m3uName
						merged.Channels = append(merged.Channels, ch)

						break
					}
				}
			}

			// Merge programs (always, even if channel was already matched by earlier EPG).
			primaryID := m3uToEPGID[m3uName]

			for _, prog := range r.EPG.Programs {
				if prog.Channel != epgID {
					continue
				}

				// Remap to primary EPG ID.
				remapped := prog
				remapped.Channel = primaryID

				// Check for time overlap with existing programs.
				if !hasOverlap(channelPrograms[primaryID], remapped) {
					channelPrograms[primaryID] = append(channelPrograms[primaryID], remapped)
				}
			}
		}
	}

	// Flatten programs.
	for _, progs := range channelPrograms {
		merged.Programs = append(merged.Programs, progs...)
	}

	return merged
}

// hasOverlap checks if a program overlaps with existing programs.
// Programs overlap if they have the same start time (duplicate).
func hasOverlap(existing []Programme, newProg Programme) bool {
	for _, p := range existing {
		// Same start time means duplicate - skip.
		if p.Start == newProg.Start {
			return true
		}
	}

	return false
}

// AddFakeChannels adds fake EPG channel entries for M3U channels not matched by any EPG.
func AddFakeChannels(
	log logrus.FieldLogger,
	epgData *TV,
	m3uChannels []m3u.Channel,
	channelMap map[string]string,
) *TV {
	// Build set of matched M3U names from channelMap values.
	matchedM3UNames := make(map[string]bool, len(channelMap))

	for _, m3uName := range channelMap {
		matchedM3UNames[m3uName] = true
	}

	// Build set of channels that have programs.
	channelsWithPrograms := make(map[string]bool, len(epgData.Programs))

	for _, prog := range epgData.Programs {
		channelsWithPrograms[prog.Channel] = true
	}

	// Build category map.
	categoryMap := buildCategoryMap(m3uChannels)

	// Generate fake channels for unmatched M3U channels.
	fakeChannels := make([]Channel, 0, len(m3uChannels))
	newChannelMap := make(map[string]string, len(channelMap))

	for k, v := range channelMap {
		newChannelMap[k] = v
	}

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
		newChannelMap[channelID] = m3uChannel.Name
	}

	if len(fakeChannels) > 0 {
		log.WithField("count", len(fakeChannels)).Info("Created EPG channel entries for unmatched M3U channels (no guide data)")
	}

	// Combine existing channels with fake channels.
	allChannels := make([]Channel, 0, len(epgData.Channels)+len(fakeChannels))
	allChannels = append(allChannels, epgData.Channels...)
	allChannels = append(allChannels, fakeChannels...)

	// Generate fake programs for channels without program data.
	fakePrograms := generateFakePrograms(allChannels, channelsWithPrograms, categoryMap, newChannelMap)

	allPrograms := make([]Programme, 0, len(epgData.Programs)+len(fakePrograms))
	allPrograms = append(allPrograms, epgData.Programs...)
	allPrograms = append(allPrograms, fakePrograms...)

	return &TV{
		XMLName:  epgData.XMLName,
		Channels: allChannels,
		Programs: allPrograms,
	}
}
