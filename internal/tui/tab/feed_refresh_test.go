package tab

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/EugeneShtoka/yt-tui/internal/domain"
)

type manualFeedBackend struct {
	fakeBackend
	fetches int
	clears  int
}

func (b *manualFeedBackend) Recommended(context.Context) ([]domain.Video, error) {
	b.fetches++
	return []domain.Video{{ID: "fresh", Title: "Fresh video"}}, nil
}

func (b *manualFeedBackend) ClearRecommended(context.Context) error {
	b.clears++
	return nil
}

// Drain the Feed's commands, including follow-up commands produced by Update.
func settleFeed(f Feed, cmd tea.Cmd) Feed {
	if cmd == nil {
		return f
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, child := range batch {
			f = settleFeed(f, child)
		}
		return f
	}
	m, next := f.Update(msg)
	return settleFeed(m.(Feed), next)
}

func TestFeedLoadsCacheWithoutFetching(t *testing.T) {
	for _, name := range []string{"cached", "empty", "error"} {
		t.Run(name, func(t *testing.T) {
			backend := &manualFeedBackend{}
			backend.getFeedCache = func(context.Context, string) ([]domain.Video, error) {
				switch name {
				case "cached":
					return []domain.Video{{ID: "cached", Title: "Cached video"}}, nil
				case "error":
					return nil, errors.New("cache unreadable")
				default:
					return nil, nil
				}
			}
			f := NewFeed(t.Context(), backend, testKeys(), false, FeedOpts{})
			f, _ = updateFeed(f, sized(80, 24))
			f = settleFeed(f, f.Init())
			if backend.fetches != 0 || f.Loading() {
				t.Fatalf("startup: fetches=%d, loading=%v", backend.fetches, f.Loading())
			}
			if name == "cached" && f.feed.Len() != 1 {
				t.Fatal("cached result was not displayed")
			}
			if name == "empty" && !strings.Contains(f.View().Content, "refresh") {
				t.Fatal("empty feed should explain how to refresh")
			}
			f, cmd := updateFeed(f, tea.KeyPressMsg{Text: "r"})
			f = settleFeed(f, cmd)
			if backend.fetches != 1 || f.Loading() || len(f.recVideos) == 0 {
				t.Fatalf("manual refresh: fetches=%d, loading=%v, videos=%v", backend.fetches, f.Loading(), f.recVideos)
			}
			f, cmd = updateFeed(f, tea.KeyPressMsg{Text: "R"})
			_ = settleFeed(f, cmd)
			if backend.fetches != 2 || backend.clears != 1 {
				t.Fatalf("force refresh: fetches=%d, clears=%d", backend.fetches, backend.clears)
			}
		})
	}
}

func TestEnteringRecommendedModeDoesNotFetch(t *testing.T) {
	backend := &manualFeedBackend{}
	f := NewFeed(t.Context(), backend, testKeys(), false, FeedOpts{Mode: "subscribed"})
	f = settleFeed(f, f.Init())
	for _, text := range []string{"M", "k", "enter"} {
		updated, cmd := updateFeed(f, tea.KeyPressMsg{Text: text})
		f = settleFeed(updated, cmd)
	}
	if f.mode != feedRecommended || backend.fetches != 0 || f.Loading() {
		t.Fatalf("entering Recommended: mode=%v, fetches=%d, loading=%v", f.mode, backend.fetches, f.Loading())
	}
}
