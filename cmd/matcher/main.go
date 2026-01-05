// Package main provides a CLI tool for debugging EPG channel matching.
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/savid/iptv/internal/epg"
	"github.com/savid/iptv/internal/m3u"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const noProgramsMsg = "NO PROGRAMS"

var (
	m3uPath  string
	epgPath  string
	logLevel string
	log      = logrus.New()
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "matcher",
		Short: "Debug EPG channel matching",
		Long: `A debugging tool to analyze how M3U channels match to EPG data.

Outputs detailed information about:
- Which channels matched and by what strategy (tvg-id, display-name, normalized)
- Which channels failed to match and why
- Close matches that almost matched
- Summary statistics

Examples:
  # Using local files
  go run cmd/matcher/main.go --m3u testdata/channels.m3u --epg testdata/epg.xml

  # Using URLs
  go run cmd/matcher/main.go --m3u https://example.com/playlist.m3u --epg https://epg.example.com/epg.xml`,
		RunE: run,
	}

	rootCmd.Flags().StringVar(&m3uPath, "m3u", "", "Path or URL to M3U playlist (required)")
	rootCmd.Flags().StringVar(&epgPath, "epg", "", "Path or URL to EPG XML (required)")
	rootCmd.Flags().StringVar(&logLevel, "log-level", "debug", "Log level (debug, info, warn, error)")

	if err := rootCmd.MarkFlagRequired("m3u"); err != nil {
		log.WithError(err).Fatal("Failed to mark m3u flag as required")
	}

	if err := rootCmd.MarkFlagRequired("epg"); err != nil {
		log.WithError(err).Fatal("Failed to mark epg flag as required")
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// loadData fetches data from a URL or reads from a local file.
func loadData(path string) ([]byte, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		resp, err := http.Get(path) //nolint:gosec,noctx // User-provided URL for CLI tool
		if err != nil {
			return nil, fmt.Errorf("failed to fetch URL: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP request failed with status: %s", resp.Status)
		}

		return io.ReadAll(resp.Body)
	}

	return os.ReadFile(path)
}

func run(cmd *cobra.Command, args []string) error {
	// Configure logger
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	}

	log.SetLevel(level)
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339,
	})

	// Load M3U
	log.WithField("source", m3uPath).Info("Loading M3U")

	m3uData, err := loadData(m3uPath)
	if err != nil {
		return fmt.Errorf("failed to load M3U: %w", err)
	}

	m3uChannels, err := m3u.Parse(m3uData)
	if err != nil {
		return fmt.Errorf("failed to parse M3U: %w", err)
	}

	log.WithField("count", len(m3uChannels)).Info("Parsed M3U channels")

	// Load EPG
	log.WithField("source", epgPath).Info("Loading EPG")

	epgData, err := loadData(epgPath)
	if err != nil {
		return fmt.Errorf("failed to load EPG: %w", err)
	}

	epgTV, err := epg.Parse(epgData)
	if err != nil {
		return fmt.Errorf("failed to parse EPG: %w", err)
	}

	log.WithFields(logrus.Fields{
		"channels":   len(epgTV.Channels),
		"programmes": len(epgTV.Programs),
	}).Info("Parsed EPG data")

	// Run the actual Filter function from internal/epg
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("RUNNING EPG FILTER (internal/epg.Filter)")
	fmt.Println(strings.Repeat("=", 80))

	filteredEPG, channelIDMap := epg.Filter(log, epgTV, m3uChannels)

	// Analyze and print results
	analyzeResults(m3uChannels, epgTV, filteredEPG, channelIDMap)

	return nil
}

// analyzeResults prints detailed matching analysis.
func analyzeResults(m3uChannels []m3u.Channel, originalEPG, filteredEPG *epg.TV, channelIDMap map[string]string) {
	// Build program count map
	programCount := make(map[string]int, len(filteredEPG.Channels))

	for _, prog := range filteredEPG.Programs {
		programCount[prog.Channel]++
	}

	// Build reverse map: M3U name -> EPG channel
	m3uToEPG := make(map[string]*epg.Channel, len(filteredEPG.Channels))

	for i := range filteredEPG.Channels {
		ch := &filteredEPG.Channels[i]
		if m3uName, ok := channelIDMap[ch.ID]; ok {
			m3uToEPG[m3uName] = ch
		}
	}

	// Categorize results
	var matched, unmatched []m3u.Channel

	for _, m3uCh := range m3uChannels {
		if _, ok := m3uToEPG[m3uCh.Name]; ok {
			matched = append(matched, m3uCh)
		} else {
			unmatched = append(unmatched, m3uCh)
		}
	}

	// Print matched channels
	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Printf("MATCHED CHANNELS (%d/%d)\n", len(matched), len(m3uChannels))
	fmt.Println(strings.Repeat("-", 80))

	// Group by match type (infer from tvg-id presence and name comparison)
	var byTVGID, byDisplayName, byNormalized []m3u.Channel

	for _, m3uCh := range matched {
		epgCh := m3uToEPG[m3uCh.Name]

		if m3uCh.TVGID != "" && epgCh.ID == m3uCh.TVGID {
			byTVGID = append(byTVGID, m3uCh)
		} else if m3uCh.Name == epgCh.DisplayName {
			byDisplayName = append(byDisplayName, m3uCh)
		} else {
			byNormalized = append(byNormalized, m3uCh)
		}
	}

	// Print by category
	if len(byTVGID) > 0 {
		fmt.Printf("\n  [TVG-ID] (%d channels)\n", len(byTVGID))

		for _, m3uCh := range byTVGID {
			epgCh := m3uToEPG[m3uCh.Name]
			progCount := programCount[epgCh.ID]
			programInfo := fmt.Sprintf("%d programs", progCount)

			if progCount == 0 {
				programInfo = noProgramsMsg
			}

			fmt.Printf("    %-40s -> %-30s [%s]\n",
				truncate(m3uCh.Name, 40),
				truncate(epgCh.DisplayName, 30),
				programInfo,
			)
		}
	}

	if len(byDisplayName) > 0 {
		fmt.Printf("\n  [DISPLAY-NAME] (%d channels)\n", len(byDisplayName))

		for _, m3uCh := range byDisplayName {
			epgCh := m3uToEPG[m3uCh.Name]
			progCount := programCount[epgCh.ID]
			programInfo := fmt.Sprintf("%d programs", progCount)

			if progCount == 0 {
				programInfo = noProgramsMsg
			}

			fmt.Printf("    %-40s -> %-30s [%s]\n",
				truncate(m3uCh.Name, 40),
				truncate(epgCh.DisplayName, 30),
				programInfo,
			)
		}
	}

	if len(byNormalized) > 0 {
		fmt.Printf("\n  [NORMALIZED] (%d channels)\n", len(byNormalized))

		for _, m3uCh := range byNormalized {
			epgCh := m3uToEPG[m3uCh.Name]
			progCount := programCount[epgCh.ID]
			programInfo := fmt.Sprintf("%d programs", progCount)

			if progCount == 0 {
				programInfo = noProgramsMsg
			}

			fmt.Printf("    %-40s -> %-30s [%s]\n",
				truncate(m3uCh.Name, 40),
				truncate(epgCh.DisplayName, 30),
				programInfo,
			)
		}
	}

	// Print unmatched channels
	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Printf("UNMATCHED CHANNELS (%d/%d)\n", len(unmatched), len(m3uChannels))
	fmt.Println(strings.Repeat("-", 80))

	if len(unmatched) == 0 {
		fmt.Println("  All channels matched!")
	} else {
		for _, m3uCh := range unmatched {
			fmt.Printf("\n  %s\n", m3uCh.Name)
			fmt.Printf("    tvg-id: %q\n", m3uCh.TVGID)

			closeMatches := findClosestMatches(m3uCh.Name, originalEPG.Channels)
			if len(closeMatches) > 0 {
				fmt.Println("    close matches in EPG:")

				for _, match := range closeMatches {
					fmt.Printf("      - %s\n", match)
				}
			} else {
				fmt.Println("    no close matches found")
			}
		}
	}

	// Print summary
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("SUMMARY")
	fmt.Println(strings.Repeat("=", 80))

	matchRate := float64(len(matched)) / float64(len(m3uChannels)) * 100

	fmt.Printf("  Total M3U channels:  %d\n", len(m3uChannels))
	fmt.Printf("  Matched:             %d (%.1f%%)\n", len(matched), matchRate)
	fmt.Printf("  Unmatched:           %d\n", len(unmatched))
	fmt.Println()
	fmt.Printf("  By strategy:\n")
	fmt.Printf("    tvg-id:       %d\n", len(byTVGID))
	fmt.Printf("    display-name: %d\n", len(byDisplayName))
	fmt.Printf("    normalized:   %d\n", len(byNormalized))

	// Count channels with real programs (not fake)
	withPrograms := 0

	for _, m3uCh := range matched {
		epgCh := m3uToEPG[m3uCh.Name]
		if programCount[epgCh.ID] > 1 { // More than just the fake program
			withPrograms++
		}
	}

	fmt.Println()
	fmt.Printf("  Matched with programs: %d\n", withPrograms)
	fmt.Printf("  Matched without programs: %d\n", len(matched)-withPrograms)

	fmt.Println(strings.Repeat("=", 80))
}

// findClosestMatches finds EPG channels with similar names using simple token matching.
func findClosestMatches(m3uName string, epgChannels []epg.Channel) []string {
	// Simple tokenization for matching
	m3uLower := strings.ToLower(m3uName)
	tokens := strings.Fields(m3uLower)

	if len(tokens) == 0 {
		return nil
	}

	type scored struct {
		name  string
		score int
	}

	candidates := make([]scored, 0, 10)

	for _, ch := range epgChannels {
		epgLower := strings.ToLower(ch.DisplayName)
		epgTokens := strings.Fields(epgLower)

		// Count matching tokens
		matches := 0

		for _, t1 := range tokens {
			for _, t2 := range epgTokens {
				if t1 == t2 {
					matches++

					break
				}
			}
		}

		if matches > 0 {
			candidates = append(candidates, scored{
				name:  ch.DisplayName,
				score: matches,
			})
		}
	}

	// Sort by score (descending)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	// Return top 5
	result := make([]string, 0, 5)

	for i := 0; i < len(candidates) && i < 5; i++ {
		result = append(result, candidates[i].name)
	}

	return result
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	return s[:maxLen-3] + "..."
}
