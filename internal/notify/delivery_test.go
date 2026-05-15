package notify

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/steveyegge/gastown/internal/nudge"
	"github.com/steveyegge/gastown/internal/tmux"
)

func TestDeliverQueueModePreservesMetadata(t *testing.T) {
	townRoot := t.TempDir()
	sessionName := "gt-crew-notify-queue"

	result, err := Deliver(Request{
		Tmux:        tmux.NewTmuxWithSocket("unused"),
		TownRoot:    townRoot,
		SessionName: sessionName,
		Message:     "mail ready",
		Sender:      "gastown/witness",
		Priority:    nudge.PriorityUrgent,
		Kind:        "mail",
		ThreadID:    "hq-abc123",
		Severity:    "high",
		Mode:        ModeQueue,
	})
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if !result.Queued || result.Direct {
		t.Fatalf("result = %+v, want queued-only delivery", result)
	}

	drained, err := nudge.Drain(townRoot, sessionName)
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if len(drained) != 1 {
		t.Fatalf("Drain returned %d nudges, want 1", len(drained))
	}
	got := drained[0]
	if got.Sender != "gastown/witness" || got.Message != "mail ready" || got.Priority != nudge.PriorityUrgent || got.Kind != "mail" || got.ThreadID != "hq-abc123" || got.Severity != "high" {
		t.Fatalf("queued nudge metadata = %+v", got)
	}
}

func TestWaitOptsRequireEmptyInputForHumanFacingSessions(t *testing.T) {
	tests := []struct {
		sessionName string
		want        bool
	}{
		{"hq-mayor", true},
		{"gt-mayor", true},
		{"hq-deacon", false},
		{"gt-crew-Dementus", false},
		{"gt-witness", false},
	}

	for _, tt := range tests {
		t.Run(tt.sessionName, func(t *testing.T) {
			if got := WaitOpts(tt.sessionName).RequireEmptyInput; got != tt.want {
				t.Fatalf("WaitOpts(%q).RequireEmptyInput = %v, want %v", tt.sessionName, got, tt.want)
			}
		})
	}
}

func TestShouldSkipDrainUntilIdle(t *testing.T) {
	if !ShouldSkipDrainUntilIdle(true, tmux.ErrIdleTimeout) {
		t.Fatal("prompt-aware delivery should wait for a clean idle prompt")
	}
	if ShouldSkipDrainUntilIdle(false, tmux.ErrIdleTimeout) {
		t.Fatal("promptless delivery should preserve best-effort queue draining")
	}
}

func TestWatchAndDeliverQueuedExitsOnEmptyQueue(t *testing.T) {
	townRoot := t.TempDir()
	done := make(chan struct{})
	go func() {
		WatchAndDeliverQueued(WatchRequest{
			TownRoot:     townRoot,
			SessionName:  "gt-crew-empty-queue",
			Timeout:      500 * time.Millisecond,
			PollInterval: 25 * time.Millisecond,
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WatchAndDeliverQueued did not exit after finding an empty queue")
	}
}

func TestDeliverRefusesDirectFallbackForHumanFacingQueueFailure(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	t.Setenv("GT_TEST_NUDGE_LOG", "")

	socket := "gt-notify-" + time.Now().Format("150405000000")
	sessionName := "gt-mayor"
	cmd := exec.Command("tmux", "-L", socket, "new-session", "-d", "-s", sessionName, "sleep 300")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create tmux session: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		_ = exec.Command("tmux", "-L", socket, "kill-server").Run()
	})

	townRootFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(townRootFile, []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := Deliver(Request{
		Tmux:         tmux.NewTmuxWithSocket(socket),
		TownRoot:     townRootFile,
		SessionName:  sessionName,
		Message:      "do not splice into human input",
		Sender:       "test",
		Mode:         ModeWaitIdle,
		DirectFormat: DirectQueuedReminder,
		WaitTimeout:  250 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("Deliver succeeded; want queue failure to block human-facing direct fallback")
	}
	if !strings.Contains(err.Error(), "refusing direct delivery") {
		t.Fatalf("Deliver error = %v, want refusing-direct-delivery error", err)
	}
	if errors.Is(err, tmux.ErrSessionNotFound) {
		t.Fatalf("Deliver error = %v, session should exist", err)
	}
}
