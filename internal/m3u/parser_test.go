package m3u

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse_ValidPlaylist(t *testing.T) {
	input := `#EXTM3U
#EXTINF:-1 tvg-id="espn.us" tvg-name="ESPN" tvg-logo="http://logo.example.com/espn.png" group-title="US Sports",ESPN
http://stream.example.com/12345

#EXTINF:-1 tvg-id="hbo.us" tvg-name="HBO" tvg-logo="http://logo.example.com/hbo.png" group-title="US Movies",HBO
http://stream.example.com/12346
`
	channels, err := Parse([]byte(input))
	require.NoError(t, err)
	require.Len(t, channels, 2)

	require.Equal(t, "ESPN", channels[0].Name)
	require.Equal(t, "http://stream.example.com/12345", channels[0].URL)
	require.Equal(t, "espn.us", channels[0].TVGID)
	require.Equal(t, "ESPN", channels[0].TVGName)
	require.Equal(t, "http://logo.example.com/espn.png", channels[0].TVGLogo)
	require.Equal(t, "US Sports", channels[0].Group)

	require.Equal(t, "HBO", channels[1].Name)
	require.Equal(t, "http://stream.example.com/12346", channels[1].URL)
	require.Equal(t, "hbo.us", channels[1].TVGID)
	require.Equal(t, "HBO", channels[1].TVGName)
	require.Equal(t, "http://logo.example.com/hbo.png", channels[1].TVGLogo)
	require.Equal(t, "US Movies", channels[1].Group)
}

func TestParse_ExtractAttributes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Channel
	}{
		{
			name: "all attributes",
			input: `#EXTM3U
#EXTINF:-1 tvg-id="fox.sports.1" tvg-name="FOX Sports 1" tvg-logo="http://logo.example.com/fox.png" group-title="Australia",FOX Sports 1
http://stream.example.com/1`,
			expected: Channel{
				Name:    "FOX Sports 1",
				URL:     "http://stream.example.com/1",
				TVGID:   "fox.sports.1",
				TVGName: "FOX Sports 1",
				TVGLogo: "http://logo.example.com/fox.png",
				Group:   "Australia",
			},
		},
		{
			name: "missing tvg-logo",
			input: `#EXTM3U
#EXTINF:-1 tvg-name="CNN" group-title="News",CNN
http://stream.example.com/cnn`,
			expected: Channel{
				Name:    "CNN",
				URL:     "http://stream.example.com/cnn",
				TVGID:   "",
				TVGName: "CNN",
				TVGLogo: "",
				Group:   "News",
			},
		},
		{
			name: "missing group-title",
			input: `#EXTM3U
#EXTINF:-1 tvg-name="BBC" tvg-logo="http://logo.example.com/bbc.png",BBC
http://stream.example.com/bbc`,
			expected: Channel{
				Name:    "BBC",
				URL:     "http://stream.example.com/bbc",
				TVGID:   "",
				TVGName: "BBC",
				TVGLogo: "http://logo.example.com/bbc.png",
				Group:   "",
			},
		},
		{
			name: "no attributes",
			input: `#EXTM3U
#EXTINF:-1,Local Channel
http://stream.example.com/local`,
			expected: Channel{
				Name:    "Local Channel",
				URL:     "http://stream.example.com/local",
				TVGID:   "",
				TVGName: "",
				TVGLogo: "",
				Group:   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channels, err := Parse([]byte(tt.input))
			require.NoError(t, err)
			require.Len(t, channels, 1)

			require.Equal(t, tt.expected.Name, channels[0].Name)
			require.Equal(t, tt.expected.URL, channels[0].URL)
			require.Equal(t, tt.expected.TVGID, channels[0].TVGID)
			require.Equal(t, tt.expected.TVGName, channels[0].TVGName)
			require.Equal(t, tt.expected.TVGLogo, channels[0].TVGLogo)
			require.Equal(t, tt.expected.Group, channels[0].Group)
		})
	}
}

func TestParse_ChannelNameFromComma(t *testing.T) {
	input := `#EXTM3U
#EXTINF:-1 tvg-name="Short Name",This Is The Full Channel Name
http://stream.example.com/1`

	channels, err := Parse([]byte(input))
	require.NoError(t, err)
	require.Len(t, channels, 1)
	require.Equal(t, "This Is The Full Channel Name", channels[0].Name)
	require.Equal(t, "Short Name", channels[0].TVGName)
}

func TestParse_EmptyLines(t *testing.T) {
	input := `#EXTM3U

#EXTINF:-1 tvg-name="Channel1",Channel 1

http://stream.example.com/1


#EXTINF:-1 tvg-name="Channel2",Channel 2

http://stream.example.com/2

`
	channels, err := Parse([]byte(input))
	require.NoError(t, err)
	require.Len(t, channels, 2)
	require.Equal(t, "Channel 1", channels[0].Name)
	require.Equal(t, "Channel 2", channels[1].Name)
}

func TestParse_NoHeader(t *testing.T) {
	input := `#EXTINF:-1 tvg-name="Channel1",Channel 1
http://stream.example.com/1`

	channels, err := Parse([]byte(input))
	require.NoError(t, err)
	require.Len(t, channels, 1)
	require.Equal(t, "Channel 1", channels[0].Name)
}

func TestParse_ErrIncompleteChannel(t *testing.T) {
	input := `#EXTM3U
#EXTINF:-1 tvg-name="Channel1",Channel 1
http://stream.example.com/1
#EXTINF:-1 tvg-name="Channel2",Channel 2`

	_, err := Parse([]byte(input))
	require.ErrorIs(t, err, ErrIncompleteChannel)
}

func TestParse_ErrOrphanedChannel(t *testing.T) {
	input := `#EXTM3U
#EXTINF:-1 tvg-name="Channel1",Channel 1
#EXTINF:-1 tvg-name="Channel2",Channel 2
http://stream.example.com/2`

	_, err := Parse([]byte(input))
	require.ErrorIs(t, err, ErrOrphanedChannel)
}

func TestParse_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "unicode characters",
			input: `#EXTM3U
#EXTINF:-1 tvg-name="Tele Zurich",Télé Zürich
http://stream.example.com/1`,
			expected: "Télé Zürich",
		},
		{
			name: "ampersand in name",
			input: `#EXTM3U
#EXTINF:-1 tvg-name="A&E",A&E Network
http://stream.example.com/1`,
			expected: "A&E Network",
		},
		{
			name: "parentheses in name",
			input: `#EXTM3U
#EXTINF:-1 tvg-name="ESPN (HD)",ESPN (HD)
http://stream.example.com/1`,
			expected: "ESPN (HD)",
		},
		{
			name: "numbers in name",
			input: `#EXTM3U
#EXTINF:-1 tvg-name="24/7 News",24/7 Breaking News
http://stream.example.com/1`,
			expected: "24/7 Breaking News",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channels, err := Parse([]byte(tt.input))
			require.NoError(t, err)
			require.Len(t, channels, 1)
			require.Equal(t, tt.expected, channels[0].Name)
		})
	}
}

func TestParse_OriginalLine(t *testing.T) {
	input := `#EXTM3U
#EXTINF:-1 tvg-id="test" tvg-name="Test" tvg-logo="http://logo.example.com/test.png" group-title="Test Group",Test Channel
http://stream.example.com/1`

	channels, err := Parse([]byte(input))
	require.NoError(t, err)
	require.Len(t, channels, 1)
	require.Contains(t, channels[0].Original, "#EXTINF:-1")
	require.Contains(t, channels[0].Original, "tvg-id=\"test\"")
}

func TestRewrite_GeneratesValidM3U(t *testing.T) {
	channels := []Channel{
		{
			Name:    "ESPN",
			URL:     "http://stream.example.com/1",
			TVGName: "ESPN",
			TVGLogo: "http://logo.example.com/espn.png",
			Group:   "US Sports",
		},
		{
			Name:    "HBO",
			URL:     "http://stream.example.com/2",
			TVGName: "HBO",
			TVGLogo: "http://logo.example.com/hbo.png",
			Group:   "US Movies",
		},
	}

	result := Rewrite(channels, nil)

	require.Contains(t, result, "#EXTM3U")
	require.Contains(t, result, `tvg-name="ESPN"`)
	require.Contains(t, result, `tvg-logo="http://logo.example.com/espn.png"`)
	require.Contains(t, result, `group-title="US Sports"`)
	require.Contains(t, result, ",ESPN")
	require.Contains(t, result, "http://stream.example.com/1")
	require.Contains(t, result, `tvg-name="HBO"`)
	require.Contains(t, result, "http://stream.example.com/2")
}

func TestRewrite_EmptyChannels(t *testing.T) {
	result := Rewrite([]Channel{}, nil)
	require.Equal(t, "#EXTM3U\n", result)
}

func TestRewrite_RoundTrip(t *testing.T) {
	original := []Channel{
		{
			Name:    "Test Channel",
			URL:     "http://stream.example.com/test",
			TVGName: "Test",
			TVGLogo: "http://logo.example.com/test.png",
			Group:   "Test Group",
		},
	}

	rewritten := Rewrite(original, nil)
	parsed, err := Parse([]byte(rewritten))
	require.NoError(t, err)
	require.Len(t, parsed, 1)

	require.Equal(t, original[0].Name, parsed[0].Name)
	require.Equal(t, original[0].URL, parsed[0].URL)
	require.Equal(t, original[0].TVGName, parsed[0].TVGName)
	require.Equal(t, original[0].TVGLogo, parsed[0].TVGLogo)
	require.Equal(t, original[0].Group, parsed[0].Group)
}

func TestExtractAttribute(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		attr     string
		expected string
	}{
		{
			name:     "extract tvg-name",
			line:     `#EXTINF:-1 tvg-name="ESPN" tvg-logo="http://logo.example.com/espn.png"`,
			attr:     "tvg-name",
			expected: "ESPN",
		},
		{
			name:     "extract tvg-logo",
			line:     `#EXTINF:-1 tvg-name="ESPN" tvg-logo="http://logo.example.com/espn.png"`,
			attr:     "tvg-logo",
			expected: "http://logo.example.com/espn.png",
		},
		{
			name:     "extract group-title",
			line:     `#EXTINF:-1 tvg-name="ESPN" group-title="US Sports"`,
			attr:     "group-title",
			expected: "US Sports",
		},
		{
			name:     "missing attribute",
			line:     `#EXTINF:-1 tvg-name="ESPN"`,
			attr:     "tvg-logo",
			expected: "",
		},
		{
			name:     "empty attribute value",
			line:     `#EXTINF:-1 tvg-name=""`,
			attr:     "tvg-name",
			expected: "",
		},
		{
			name:     "attribute with spaces in value",
			line:     `#EXTINF:-1 tvg-name="FOX Sports 1"`,
			attr:     "tvg-name",
			expected: "FOX Sports 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAttribute(tt.line, tt.attr)
			require.Equal(t, tt.expected, result)
		})
	}
}
