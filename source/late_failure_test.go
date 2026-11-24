package main

// User story: when YouTube Music fails after the start was accepted, the
// speaker says a short approved sentence. The panel is closed to everyone but
// the voice bridge, so the sentence must not send the listener there.

import (
	"strings"
	"testing"
	"time"
)

func TestLateFailureSpeaksApprovedTextWithoutPanel(t *testing.T) {
	spoken := make(chan string, 1)
	old := failureSpeech
	failureSpeech = func(text string) error { spoken <- text; return nil }
	defer func() { failureSpeech = old }()
	options := defaultSettings()
	s := &Server{settings: &options, state: State{Status: "error"}}
	s.notifyFailure(s.generation)
	select {
	case text := <-spoken:
		if text != "Не удалось включить музыку на YouTube." || strings.Contains(text, "панел") {
			t.Fatalf("late failure said %q", text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("late failure was not spoken")
	}
}
