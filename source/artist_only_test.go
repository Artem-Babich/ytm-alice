package main

// User story: a voice bridge asks to play a band by name ("включи группу
// Порнофильмы"). The search also returns a song merely titled "Порнофильмы"
// by another artist. When the request names only the artist (artist set,
// title empty), the queue holds only that artist's songs, so "дальше" never
// jumps to someone else's song. When nothing in the results is credited to
// that artist (the catalog spells it differently), the queue stays as the
// plain search would build it.

import (
	"context"
	"net/http"
	"testing"
)

func musicItem(id, title, artist string) map[string]any {
	column := func(run map[string]any) map[string]any {
		return map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{run}}}}
	}
	return map[string]any{"musicResponsiveListItemRenderer": map[string]any{
		"playlistItemData": map[string]any{"videoId": id},
		"flexColumns": []any{
			column(map[string]any{"text": title}),
			column(map[string]any{"text": artist, "navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{"browseId": "UC" + id}}}),
		},
	}}
}

func artistServer(t *testing.T, items ...any) *Server {
	t.Helper()
	t.Setenv("YTM_ALICE_DATA", t.TempDir())
	c := newCatalog()
	c.http = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		return jsonReply(map[string]any{"contents": items}), nil
	})}
	return &Server{catalog: c}
}

func queueIDs(s *Server) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := []string{}
	for _, track := range s.state.Queue {
		ids = append(ids, track.ID)
	}
	return ids
}

func TestPlayArtistOnlyQueuesOnlyThatArtist(t *testing.T) {
	s := artistServer(t,
		musicItem("aaaaaaaaaaa", "Я так соскучился", "Порнофильмы"),
		musicItem("bbbbbbbbbbb", "Порнофильмы", "GSPD"),
		musicItem("ccccccccccc", "Это пройдёт", "Порнофильмы"),
	)
	out := s.command(context.Background(), request{Action: "play", Query: "Порнофильмы", Artist: "Порнофильмы"})
	if out.Error != "" {
		t.Fatal(out.Error)
	}
	if got := queueIDs(s); len(got) != 2 || got[0] != "aaaaaaaaaaa" || got[1] != "ccccccccccc" {
		t.Fatalf("queue %v must hold only the band's songs", got)
	}
}

func TestPlayWithoutArtistKeepsSongsMatchingByTitle(t *testing.T) {
	s := artistServer(t,
		musicItem("aaaaaaaaaaa", "Я так соскучился", "Порнофильмы"),
		musicItem("bbbbbbbbbbb", "Порнофильмы", "GSPD"),
	)
	if out := s.command(context.Background(), request{Action: "play", Query: "Порнофильмы"}); out.Error != "" {
		t.Fatal(out.Error)
	}
	if got := queueIDs(s); len(got) != 2 {
		t.Fatalf("a plain search must keep the title match too: %v", got)
	}
}

func TestPlayArtistOnlyFallsBackWhenNoSongIsCreditedToTheArtist(t *testing.T) {
	s := artistServer(t, musicItem("aaaaaaaaaaa", "Порнофильмы", "GSPD"))
	if out := s.command(context.Background(), request{Action: "play", Query: "Порнофильмы", Artist: "Порнофильмы"}); out.Error != "" {
		t.Fatal(out.Error)
	}
	if got := queueIDs(s); len(got) != 1 {
		t.Fatalf("with no song by the artist the plain result must play: %v", got)
	}
}
