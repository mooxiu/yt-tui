package tab

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/EugeneShtoka/yt-tui/internal/domain"
	tuipkg "github.com/EugeneShtoka/yt-tui/internal/tui"
)

type searchHistoryBackend struct {
	fakeBackend
	history []string
}

func (b *searchHistoryBackend) AddHistory(_ context.Context, _, _ string, query string) error {
	b.history = append(b.history, query)
	return nil
}

func TestSearchCacheReusesResultsAcrossEntryPoints(t *testing.T) {
	calls := 0
	backend := &searchHistoryBackend{}
	backend.search = func(_ context.Context, query string) ([]domain.Channel, []domain.Video, error) {
		calls++
		return []domain.Channel{{ID: query}}, []domain.Video{{ID: query}}, nil
	}
	s := NewSearch(t.Context(), backend, testKeys(), false)
	s.input.SetValue(" cats ")
	m, cmd := s.routeKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	s = asSearch(t, m)
	m, _ = s.Update(cmd())
	s = asSearch(t, m)
	expires := s.cache["cats"].expiresAt

	// Search something else, then return through recent queries and History.
	for _, query := range []string{"dogs", "cats"} {
		m, cmd = s.srchExecuteRecent(query)
		s = asSearch(t, m)
		m, _ = s.Update(cmd())
		s = asSearch(t, m)
	}
	m, cmd = s.Update(tuipkg.SearchActivateMsg{Query: "cats"})
	s = asSearch(t, m)
	m, _ = s.Update(cmd())
	s = asSearch(t, m)
	if calls != 2 {
		t.Fatalf("backend searches = %d, want 2", calls)
	}
	if s.channels[0].ID != "cats" || s.videos[0].ID != "cats" {
		t.Fatalf("cached channels/videos not restored: %v / %v", s.channels, s.videos)
	}
	if !s.cache["cats"].expiresAt.Equal(expires) {
		t.Error("cache hits extended the expiration")
	}
	if want := []string{"cats", "dogs", "cats", "cats"}; !reflect.DeepEqual(backend.history, want) {
		t.Fatalf("history = %v, want %v", backend.history, want)
	}
}

func TestSearchCacheExpiredResultsAreFetchedAgain(t *testing.T) {
	calls := 0
	s := NewSearch(t.Context(), &fakeBackend{search: func(context.Context, string) ([]domain.Channel, []domain.Video, error) {
		calls++
		return nil, []domain.Video{{ID: "fresh"}}, nil
	}}, testKeys(), false)
	s.cache["cats"] = searchCacheEntry{
		result:    srchResultMsg{query: "cats", videos: []domain.Video{{ID: "stale"}}},
		expiresAt: time.Now().Add(-time.Second),
	}
	result := s.srchCmd("cats")().(srchResultMsg)
	if calls != 1 || result.videos[0].ID != "fresh" {
		t.Fatalf("expired search was reused: calls=%d, result=%v", calls, result)
	}
}

func TestSearchCacheRetriesErrorsAndCachesEmptyResults(t *testing.T) {
	calls := 0
	s := NewSearch(t.Context(), &fakeBackend{search: func(context.Context, string) ([]domain.Channel, []domain.Video, error) {
		calls++
		if calls == 1 {
			return nil, nil, errors.New("offline")
		}
		return nil, nil, nil
	}}, testKeys(), false)
	for range 3 {
		m, _ := s.Update(s.srchCmd("empty")())
		s = asSearch(t, m)
	}
	if calls != 2 {
		t.Fatalf("backend searches = %d, want retry followed by cached empty result (2)", calls)
	}
}

func TestSearchCacheBoundsMemory(t *testing.T) {
	s := newSearchTab()
	s.cache["oldest"] = searchCacheEntry{expiresAt: time.Now().Add(time.Minute)}
	for i := range searchCacheLimit {
		s.cacheResult(srchResultMsg{query: fmt.Sprint(i)})
	}
	if len(s.cache) != searchCacheLimit {
		t.Fatalf("cache size = %d, want %d", len(s.cache), searchCacheLimit)
	}
	if _, ok := s.cache["oldest"]; ok {
		t.Error("oldest query was not evicted")
	}
	s.cache["0"] = searchCacheEntry{expiresAt: time.Now().Add(-time.Second)}
	s.cacheResult(srchResultMsg{query: "new"})
	if _, ok := s.cache["0"]; ok {
		t.Error("expired query was not pruned")
	}
}
