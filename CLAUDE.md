# IPTV Proxy

Go 1.25 IPTV proxy with HDHomeRun emulation for Plex Live TV integration.

## Tech Stack

- Language: Go 1.25
- CLI: github.com/spf13/cobra
- Logging: github.com/sirupsen/logrus
- Testing: github.com/stretchr/testify
- Linting: golangci-lint (see .golangci.yml)

## Commands

- Build: `go build -o iptv ./cmd/`
- Run: `./iptv --m3u <URL> --epg <URL> --base <URL>`
- Test: `go test ./...`
- Test with race detector: `go test -race ./...`
- Lint: `golangci-lint run --new-from-rev="origin/master"`
- Format: `gofmt -w . && goimports -w .`

## Architecture

```
cmd/main.go           # Entry point, CLI setup
cmd/matcher/main.go   # Debug tool for EPG channel matching
internal/
├── config/           # Configuration struct and validation
├── server/           # HTTP server lifecycle and routes
├── data/             # Thread-safe store, fetcher, refresher
├── hdhr/             # HDHomeRun protocol emulation
├── m3u/              # M3U playlist parser
└── epg/              # XMLTV parser and filter
```

## Testing M3U → EPG Matching

Use the matcher tool to debug channel matching between M3U playlists and EPG data:

```bash
go run cmd/matcher/main.go --m3u <M3U_URL> --epg <EPG_URL>
```

This outputs matching statistics and identifies unmatched channels to help debug why certain channels don't link to EPG data.
