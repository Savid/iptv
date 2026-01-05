# IPTV Proxy

Go-based IPTV proxy with HDHomeRun emulation for Plex Live TV integration.

## Build

```bash
go build -o iptv ./cmd/
```

## Usage

```bash
./iptv --m3u <URL> --epg <URL> --base <URL> [options]
```

### Required Flags

| Flag | Description |
|------|-------------|
| `--m3u` | M3U playlist URL |
| `--epg` | XMLTV EPG URL |
| `--base` | Base URL for stream redirects |

### Optional Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--bind` | `0.0.0.0` | Bind address |
| `--port` | `8080` | Port number |
| `--log-level` | `info` | Log level (debug, info, warn, error) |
| `--tuner-count` | `2` | Virtual tuners to advertise |
| `--device-id` | `iptv-proxy-001` | HDHomeRun device ID |
| `--device-name` | `IPTV-Proxy` | Device name shown in Plex |
| `--refresh` | `30m` | Data refresh interval |

### Examples

Basic usage:

```bash
./iptv --m3u https://provider.com/playlist.m3u \
       --epg https://provider.com/epg.xml \
       --base http://192.168.1.1:8080
```

With custom settings:

```bash
./iptv --m3u https://provider.com/playlist.m3u \
       --epg https://provider.com/epg.xml \
       --base http://iptv.local:8080 \
       --port 8080 \
       --tuner-count 4 \
       --device-name "My IPTV" \
       --log-level debug
```

## Endpoints

### HDHomeRun Discovery

- `GET /discover.json` - Device discovery
- `GET /lineup.json` - Channel lineup
- `GET /lineup_status.json` - Scan status
- `GET /auto/v{channel}` - Stream redirect

### Group-Based Virtual Devices

Channels are grouped by `group-title` attribute, each exposed as a separate device:

- `GET /{group-slug}/discover.json`
- `GET /{group-slug}/lineup.json`

### Data

- `GET /iptv.m3u` - Rewritten M3U playlist
- `GET /epg.xml` - Filtered EPG data
- `GET /health` - Health check

## Matcher Tool

Debug channel matching between M3U and EPG:

```bash
go run cmd/matcher/main.go --m3u <URL> --epg <URL>
```

Outputs matching statistics showing which channels matched via:

- **TVG-ID**: Direct `tvg-id` attribute match
- **Display Name**: Exact name match
- **Normalized**: Match after stripping region prefixes (US:, UK:) and quality suffixes (HD, FHD)

Unmatched channels show close EPG matches to help diagnose issues.

## Plex Setup

1. Start the proxy with your M3U/EPG URLs
2. In Plex, go to Settings → Live TV & DVR
3. Add device manually using `<base-url>` (eg. `http://iptv.local:8080`)
4. Plex will discover the virtual HDHomeRun tuner
5. For guide data, select XMLTV and use `<base-url>/epg.xml` (eg. `http://iptv.local:8080/epg.xml`)
