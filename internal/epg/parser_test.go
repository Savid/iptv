package epg

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse_ValidXML(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<tv>
  <channel id="espn.us">
    <display-name>ESPN</display-name>
    <icon src="http://logo.example.com/espn.png"/>
  </channel>
  <programme channel="espn.us" start="20260104120000 +0000" stop="20260104130000 +0000">
    <title>SportsCenter</title>
    <desc>Latest sports news and highlights</desc>
  </programme>
</tv>`

	tv, err := Parse([]byte(input))
	require.NoError(t, err)
	require.NotNil(t, tv)

	require.Len(t, tv.Channels, 1)
	require.Equal(t, "espn.us", tv.Channels[0].ID)
	require.Equal(t, "ESPN", tv.Channels[0].DisplayName)
	require.Equal(t, "http://logo.example.com/espn.png", tv.Channels[0].Icon.Src)

	require.Len(t, tv.Programs, 1)
	require.Equal(t, "espn.us", tv.Programs[0].Channel)
	require.Equal(t, "20260104120000 +0000", tv.Programs[0].Start)
	require.Equal(t, "20260104130000 +0000", tv.Programs[0].Stop)
	require.Equal(t, "SportsCenter", tv.Programs[0].Title)
	require.Equal(t, "Latest sports news and highlights", tv.Programs[0].Description)
}

func TestParse_MultipleChannels(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<tv>
  <channel id="espn.us">
    <display-name>ESPN</display-name>
    <icon src="http://logo.example.com/espn.png"/>
  </channel>
  <channel id="hbo.us">
    <display-name>HBO</display-name>
    <icon src="http://logo.example.com/hbo.png"/>
  </channel>
  <channel id="cnn.us">
    <display-name>CNN</display-name>
    <icon src="http://logo.example.com/cnn.png"/>
  </channel>
</tv>`

	tv, err := Parse([]byte(input))
	require.NoError(t, err)
	require.Len(t, tv.Channels, 3)

	require.Equal(t, "espn.us", tv.Channels[0].ID)
	require.Equal(t, "hbo.us", tv.Channels[1].ID)
	require.Equal(t, "cnn.us", tv.Channels[2].ID)
}

func TestParse_MultipleProgrammes(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<tv>
  <channel id="espn.us">
    <display-name>ESPN</display-name>
  </channel>
  <programme channel="espn.us" start="20260104120000 +0000" stop="20260104130000 +0000">
    <title>SportsCenter</title>
    <desc>Morning edition</desc>
  </programme>
  <programme channel="espn.us" start="20260104130000 +0000" stop="20260104140000 +0000">
    <title>NFL Live</title>
    <desc>Football analysis</desc>
  </programme>
  <programme channel="espn.us" start="20260104140000 +0000" stop="20260104150000 +0000">
    <title>First Take</title>
    <desc>Sports debate show</desc>
  </programme>
</tv>`

	tv, err := Parse([]byte(input))
	require.NoError(t, err)
	require.Len(t, tv.Programs, 3)

	require.Equal(t, "SportsCenter", tv.Programs[0].Title)
	require.Equal(t, "NFL Live", tv.Programs[1].Title)
	require.Equal(t, "First Take", tv.Programs[2].Title)
}

func TestParse_InvalidXML(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "malformed xml",
			input: `<tv><channel id="test"><display-name>Test</channel></tv>`,
		},
		{
			name:  "unclosed tag",
			input: `<tv><channel id="test"><display-name>Test</display-name>`,
		},
		{
			name:  "not xml",
			input: `this is not xml`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.input))
			require.Error(t, err)
		})
	}
}

func TestParse_EmptyTV(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<tv>
</tv>`

	tv, err := Parse([]byte(input))
	require.NoError(t, err)
	require.NotNil(t, tv)
	require.Empty(t, tv.Channels)
	require.Empty(t, tv.Programs)
}

func TestParse_ChannelWithoutIcon(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<tv>
  <channel id="test.channel">
    <display-name>Test Channel</display-name>
  </channel>
</tv>`

	tv, err := Parse([]byte(input))
	require.NoError(t, err)
	require.Len(t, tv.Channels, 1)
	require.Equal(t, "", tv.Channels[0].Icon.Src)
}

func TestParse_ProgrammeWithoutDescription(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<tv>
  <channel id="test.channel">
    <display-name>Test</display-name>
  </channel>
  <programme channel="test.channel" start="20260104120000 +0000" stop="20260104130000 +0000">
    <title>Test Show</title>
  </programme>
</tv>`

	tv, err := Parse([]byte(input))
	require.NoError(t, err)
	require.Len(t, tv.Programs, 1)
	require.Equal(t, "Test Show", tv.Programs[0].Title)
	require.Equal(t, "", tv.Programs[0].Description)
}

func TestMarshal_GeneratesValidXML(t *testing.T) {
	tv := &TV{
		Channels: []Channel{
			{
				ID:          "espn.us",
				DisplayName: "ESPN",
				Icon:        Icon{Src: "http://logo.example.com/espn.png"},
			},
		},
		Programs: []Programme{
			{
				Channel:     "espn.us",
				Start:       "20260104120000 +0000",
				Stop:        "20260104130000 +0000",
				Title:       "SportsCenter",
				Description: "Sports news",
			},
		},
	}

	data, err := Marshal(tv)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	output := string(data)
	require.Contains(t, output, `<channel id="espn.us">`)
	require.Contains(t, output, `<display-name>ESPN</display-name>`)
	require.Contains(t, output, `<icon src="http://logo.example.com/espn.png">`)
	require.Contains(t, output, `<programme channel="espn.us"`)
	require.Contains(t, output, `start="20260104120000 +0000"`)
	require.Contains(t, output, `stop="20260104130000 +0000"`)
	require.Contains(t, output, `<title>SportsCenter</title>`)
	require.Contains(t, output, `<desc>Sports news</desc>`)
}

func TestMarshal_IncludesHeader(t *testing.T) {
	tv := &TV{}

	data, err := Marshal(tv)
	require.NoError(t, err)

	output := string(data)
	require.True(t, strings.HasPrefix(output, `<?xml version="1.0" encoding="UTF-8"?>`))
}

func TestMarshal_EmptyTV(t *testing.T) {
	tv := &TV{}

	data, err := Marshal(tv)
	require.NoError(t, err)
	require.NotEmpty(t, data)
	require.Contains(t, string(data), "<tv>")
}

func TestRoundTrip(t *testing.T) {
	original := &TV{
		Channels: []Channel{
			{
				ID:          "channel1",
				DisplayName: "Channel One",
				Icon:        Icon{Src: "http://logo.example.com/1.png"},
			},
			{
				ID:          "channel2",
				DisplayName: "Channel Two",
				Icon:        Icon{Src: "http://logo.example.com/2.png"},
			},
		},
		Programs: []Programme{
			{
				Channel:     "channel1",
				Start:       "20260104100000 +0000",
				Stop:        "20260104110000 +0000",
				Title:       "Morning Show",
				Description: "Start your day",
			},
			{
				Channel:     "channel2",
				Start:       "20260104100000 +0000",
				Stop:        "20260104120000 +0000",
				Title:       "Movie",
				Description: "Feature film",
			},
		},
	}

	data, err := Marshal(original)
	require.NoError(t, err)

	parsed, err := Parse(data)
	require.NoError(t, err)

	require.Len(t, parsed.Channels, len(original.Channels))
	require.Len(t, parsed.Programs, len(original.Programs))

	for i, ch := range original.Channels {
		require.Equal(t, ch.ID, parsed.Channels[i].ID)
		require.Equal(t, ch.DisplayName, parsed.Channels[i].DisplayName)
		require.Equal(t, ch.Icon.Src, parsed.Channels[i].Icon.Src)
	}

	for i, prog := range original.Programs {
		require.Equal(t, prog.Channel, parsed.Programs[i].Channel)
		require.Equal(t, prog.Start, parsed.Programs[i].Start)
		require.Equal(t, prog.Stop, parsed.Programs[i].Stop)
		require.Equal(t, prog.Title, parsed.Programs[i].Title)
		require.Equal(t, prog.Description, parsed.Programs[i].Description)
	}
}

func TestParse_SpecialCharacters(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<tv>
  <channel id="test.channel">
    <display-name>Channel &amp; News</display-name>
  </channel>
  <programme channel="test.channel" start="20260104120000 +0000" stop="20260104130000 +0000">
    <title>Show &lt;Special&gt;</title>
    <desc>Description with &quot;quotes&quot;</desc>
  </programme>
</tv>`

	tv, err := Parse([]byte(input))
	require.NoError(t, err)
	require.Len(t, tv.Channels, 1)
	require.Equal(t, "Channel & News", tv.Channels[0].DisplayName)
	require.Equal(t, "Show <Special>", tv.Programs[0].Title)
	require.Equal(t, `Description with "quotes"`, tv.Programs[0].Description)
}
