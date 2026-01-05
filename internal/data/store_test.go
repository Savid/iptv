package data

import (
	"sync"
	"testing"
	"time"

	"github.com/savid/iptv/internal/epg"
	"github.com/savid/iptv/internal/m3u"
	"github.com/stretchr/testify/require"
)

func TestNewStore(t *testing.T) {
	store := NewStore()

	require.NotNil(t, store)
	require.NotNil(t, store.channelMap)
	require.Empty(t, store.channelMap)
	require.Nil(t, store.m3uChannels)
	require.Nil(t, store.epgData)
	require.True(t, store.lastSync.IsZero())
}

func TestSetGetM3U(t *testing.T) {
	store := NewStore()

	channels := []m3u.Channel{
		{Name: "ESPN", URL: "http://stream.example.com/1"},
		{Name: "HBO", URL: "http://stream.example.com/2"},
	}

	store.SetM3U(channels)

	gotChannels, ok := store.GetM3U()
	require.True(t, ok)
	require.Equal(t, channels, gotChannels)
}

func TestSetGetEPG(t *testing.T) {
	store := NewStore()

	epgData := &epg.TV{
		Channels: []epg.Channel{
			{ID: "espn.us", DisplayName: "ESPN"},
		},
		Programs: []epg.Programme{
			{Channel: "espn.us", Title: "SportsCenter"},
		},
	}
	channelMap := map[string]string{
		"espn.us": "ESPN",
	}

	store.SetEPG(epgData, channelMap)

	gotEPG, gotChannelMap, ok := store.GetEPG()
	require.True(t, ok)
	require.Equal(t, epgData, gotEPG)
	require.Equal(t, channelMap, gotChannelMap)
}

func TestGetM3U_NotSet(t *testing.T) {
	store := NewStore()

	channels, ok := store.GetM3U()
	require.False(t, ok)
	require.Nil(t, channels)
}

func TestGetEPG_NotSet(t *testing.T) {
	store := NewStore()

	data, channelMap, ok := store.GetEPG()
	require.False(t, ok)
	require.Nil(t, data)
	require.Nil(t, channelMap)
}

func TestLastSync(t *testing.T) {
	store := NewStore()

	require.True(t, store.LastSync().IsZero())

	before := time.Now()

	store.SetM3U([]m3u.Channel{{Name: "Test"}})

	after := time.Now()

	lastSync := store.LastSync()
	require.False(t, lastSync.IsZero())
	require.True(t, lastSync.After(before) || lastSync.Equal(before))
	require.True(t, lastSync.Before(after) || lastSync.Equal(after))
}

func TestLastSync_UpdatedBySetEPG(t *testing.T) {
	store := NewStore()

	before := time.Now()

	store.SetEPG(&epg.TV{}, map[string]string{})

	after := time.Now()

	lastSync := store.LastSync()
	require.False(t, lastSync.IsZero())
	require.True(t, lastSync.After(before) || lastSync.Equal(before))
	require.True(t, lastSync.Before(after) || lastSync.Equal(after))
}

func TestHasData(t *testing.T) {
	store := NewStore()

	require.False(t, store.HasData())

	store.SetM3U([]m3u.Channel{{Name: "Test"}})
	require.False(t, store.HasData())

	store.SetEPG(&epg.TV{}, map[string]string{})
	require.True(t, store.HasData())
}

func TestHasData_OnlyEPG(t *testing.T) {
	store := NewStore()

	store.SetEPG(&epg.TV{}, map[string]string{})
	require.False(t, store.HasData())
}

func TestConcurrentAccess(t *testing.T) {
	store := NewStore()

	var wg sync.WaitGroup

	iterations := 100

	wg.Add(4)

	go func() {
		defer wg.Done()

		for i := 0; i < iterations; i++ {
			store.SetM3U([]m3u.Channel{{Name: "Test"}})
		}
	}()

	go func() {
		defer wg.Done()

		for i := 0; i < iterations; i++ {
			store.GetM3U()
		}
	}()

	go func() {
		defer wg.Done()

		for i := 0; i < iterations; i++ {
			store.SetEPG(&epg.TV{}, map[string]string{})
		}
	}()

	go func() {
		defer wg.Done()

		for i := 0; i < iterations; i++ {
			store.GetEPG()
		}
	}()

	wg.Wait()
}

func TestConcurrentReadWrite(t *testing.T) {
	store := NewStore()

	var wg sync.WaitGroup

	done := make(chan struct{})

	wg.Add(1)

	go func() {
		defer wg.Done()

		for {
			select {
			case <-done:
				return
			default:
				store.SetM3U([]m3u.Channel{
					{Name: "Channel", URL: "http://example.com"},
				})
			}
		}
	}()

	wg.Add(1)

	go func() {
		defer wg.Done()

		for {
			select {
			case <-done:
				return
			default:
				channels, ok := store.GetM3U()
				if ok && len(channels) > 0 {
					_ = channels[0].Name
				}
			}
		}
	}()

	wg.Add(1)

	go func() {
		defer wg.Done()

		for {
			select {
			case <-done:
				return
			default:
				_ = store.HasData()
				_ = store.LastSync()
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)
	close(done)
	wg.Wait()
}

func TestUpdateM3UData(t *testing.T) {
	store := NewStore()

	channels1 := []m3u.Channel{{Name: "Channel1"}}
	store.SetM3U(channels1)

	channels2 := []m3u.Channel{{Name: "Channel2"}, {Name: "Channel3"}}
	store.SetM3U(channels2)

	gotChannels, ok := store.GetM3U()
	require.True(t, ok)
	require.Len(t, gotChannels, 2)
	require.Equal(t, "Channel2", gotChannels[0].Name)
	require.Equal(t, "Channel3", gotChannels[1].Name)
}

func TestUpdateEPGData(t *testing.T) {
	store := NewStore()

	epg1 := &epg.TV{Channels: []epg.Channel{{ID: "ch1"}}}
	store.SetEPG(epg1, map[string]string{"ch1": "Channel 1"})

	epg2 := &epg.TV{Channels: []epg.Channel{{ID: "ch2"}, {ID: "ch3"}}}
	store.SetEPG(epg2, map[string]string{"ch2": "Channel 2", "ch3": "Channel 3"})

	gotEPG, gotMap, ok := store.GetEPG()
	require.True(t, ok)
	require.Len(t, gotEPG.Channels, 2)
	require.Len(t, gotMap, 2)
	require.Equal(t, "Channel 2", gotMap["ch2"])
	require.Equal(t, "Channel 3", gotMap["ch3"])
}

func TestGetGroups(t *testing.T) {
	store := NewStore()

	channels := []m3u.Channel{
		{Name: "ESPN", Group: "Sports"},
		{Name: "Fox Sports", Group: "Sports"},
		{Name: "HBO", Group: "Movies"},
		{Name: "CNN", Group: "News"},
		{Name: "No Group", Group: ""},
	}
	store.SetM3U(channels)

	groups := store.GetGroups()

	require.Len(t, groups, 3)
	require.Equal(t, []string{"Movies", "News", "Sports"}, groups)
}

func TestGetGroups_Empty(t *testing.T) {
	store := NewStore()

	groups := store.GetGroups()

	require.Empty(t, groups)
}

func TestGetGroups_NoGroups(t *testing.T) {
	store := NewStore()

	channels := []m3u.Channel{
		{Name: "Channel 1", Group: ""},
		{Name: "Channel 2", Group: ""},
	}
	store.SetM3U(channels)

	groups := store.GetGroups()

	require.Empty(t, groups)
}

func TestGetChannelsByGroup(t *testing.T) {
	store := NewStore()

	channels := []m3u.Channel{
		{Name: "ESPN", Group: "Sports"},
		{Name: "Fox Sports", Group: "Sports"},
		{Name: "HBO", Group: "Movies"},
		{Name: "CNN", Group: "News"},
	}
	store.SetM3U(channels)

	sportsChannels, ok := store.GetChannelsByGroup("Sports")
	require.True(t, ok)
	require.Len(t, sportsChannels, 2)
	require.Equal(t, "ESPN", sportsChannels[0].Name)
	require.Equal(t, "Fox Sports", sportsChannels[1].Name)

	moviesChannels, ok := store.GetChannelsByGroup("Movies")
	require.True(t, ok)
	require.Len(t, moviesChannels, 1)
	require.Equal(t, "HBO", moviesChannels[0].Name)
}

func TestGetChannelsByGroup_EmptyGroup(t *testing.T) {
	store := NewStore()

	channels := []m3u.Channel{
		{Name: "ESPN", Group: "Sports"},
		{Name: "HBO", Group: "Movies"},
	}
	store.SetM3U(channels)

	allChannels, ok := store.GetChannelsByGroup("")
	require.True(t, ok)
	require.Len(t, allChannels, 2)
}

func TestGetChannelsByGroup_NotSet(t *testing.T) {
	store := NewStore()

	channels, ok := store.GetChannelsByGroup("Sports")
	require.False(t, ok)
	require.Nil(t, channels)
}

func TestGetChannelsByGroup_NonExistent(t *testing.T) {
	store := NewStore()

	store.SetM3U([]m3u.Channel{
		{Name: "ESPN", Group: "Sports"},
	})

	channels, ok := store.GetChannelsByGroup("NonExistent")
	require.True(t, ok)
	require.Empty(t, channels)
}
