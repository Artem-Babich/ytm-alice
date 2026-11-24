package main

// User story: a voice bridge on another machine already understands every
// phrase and sends ytm-alice commands over the panel API. With
// YTM_ALICE_VOICE=0 ytm-alice must not act on Alice's recognized text, so a
// phrase like «стоп» or «поставь шторы на половину» never reaches the player
// twice or as a song search. It still lowers the music while Alice speaks.

import (
	"bufio"
	"context"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

type lineRecorder struct {
	mu    sync.Mutex
	lines []string
}

func (r *lineRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sc := bufio.NewScanner(strings.NewReader(string(p)))
	for sc.Scan() {
		r.lines = append(r.lines, sc.Text())
	}
	return len(p), nil
}
func (r *lineRecorder) Close() error { return nil }
func (r *lineRecorder) has(line string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, l := range r.lines {
		if l == line {
			return true
		}
	}
	return false
}

var _ io.WriteCloser = (*lineRecorder)(nil)

func startVoiceLoop(t *testing.T, s *Server) net.Conn {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	old := voiceIPCAddress
	voiceIPCAddress = func() string { return ln.Addr().String() }
	t.Cleanup(func() { voiceIPCAddress = old })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.voiceLoop(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("voice loop did not stop")
		}
	})
	conn, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestVoiceSwitchOffIgnoresRecognizedPhrases(t *testing.T) {
	t.Setenv("YTM_ALICE_DATA", t.TempDir())
	t.Setenv("YTM_ALICE_VOICE", "0")
	s := &Server{state: State{Status: "buffering", Queue: []Track{{ID: "aaaaaaaaaaa"}}}}
	conn := startVoiceLoop(t, s)
	if err := frameWrite(conn, aliceStateFrame(1, "Алиса стоп", "stop-one", true)); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(700 * time.Millisecond))
	if _, err := frameRead(conn); err == nil {
		t.Fatal("Alice's dialog was cancelled although voice commands are off")
	}
	if got := s.snapshot().Status; got != "buffering" {
		t.Fatalf("a recognized phrase changed playback with voice commands off: %s", got)
	}
}

func TestVoiceSwitchOffStillLowersMusicWhileAliceSpeaks(t *testing.T) {
	t.Setenv("YTM_ALICE_DATA", t.TempDir())
	t.Setenv("YTM_ALICE_VOICE", "0")
	rec := &lineRecorder{}
	s := &Server{player: &nativePlayer{in: rec}}
	conn := startVoiceLoop(t, s)
	if err := frameWrite(conn, aliceStateFrame(2, "", "speak-one", false)); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(2 * time.Second)
	for !rec.has("volume 0.2") {
		select {
		case <-deadline:
			t.Fatal("music was not lowered while Alice spoke")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
