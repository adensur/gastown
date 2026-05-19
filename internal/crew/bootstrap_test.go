package crew

import (
	"strings"
	"testing"

	"github.com/steveyegge/gastown/internal/config"
)

// TestBuildStartupBootstrap_Claude verifies the bootstrap is a no-op for
// hook+prompt agents: beacon has no extra prime instruction, and no fallback
// delivery is required.
func TestBuildStartupBootstrap_Claude(t *testing.T) {
	t.Parallel()
	rc := config.RuntimeConfigFromPreset(config.AgentClaude)

	b := BuildStartupBootstrap("beads", "Jade", "start", rc)

	if strings.Contains(b.Beacon, "Run `gt prime`") {
		t.Errorf("Claude beacon should not include prime instruction (hooks handle it), got: %q", b.Beacon)
	}
	if !strings.Contains(b.Beacon, "[GAS TOWN]") {
		t.Errorf("beacon missing identity tag: %q", b.Beacon)
	}
	if b.Fallback == nil {
		t.Fatal("Fallback is nil")
	}
	if b.Fallback.IncludePrimeInBeacon || b.Fallback.SendStartupNudge || b.Fallback.SendBeaconNudge {
		t.Errorf("Claude should require no fallback delivery, got: %+v", b.Fallback)
	}
}

// TestBuildStartupBootstrap_Codex verifies codex (no hooks, has prompt) gets
// the prime instruction baked into the beacon and a delayed work-instructions
// nudge in the fallback plan. This is the gt-biy fix.
func TestBuildStartupBootstrap_Codex(t *testing.T) {
	t.Parallel()
	rc := config.RuntimeConfigFromPreset(config.AgentCodex)

	b := BuildStartupBootstrap("beads", "Jade", "start", rc)

	if !strings.Contains(b.Beacon, "Run `gt prime`") {
		t.Errorf("codex beacon must include prime instruction (no SessionStart hook to run it): %q", b.Beacon)
	}
	if !strings.Contains(b.Beacon, "[GAS TOWN] crew Jade (rig: beads)") {
		t.Errorf("beacon missing identity tag: %q", b.Beacon)
	}
	if b.Fallback == nil {
		t.Fatal("Fallback is nil")
	}
	if !b.Fallback.IncludePrimeInBeacon {
		t.Error("IncludePrimeInBeacon should be true for non-hook agent")
	}
	if !b.Fallback.SendStartupNudge {
		t.Error("SendStartupNudge should be true so work instructions arrive after gt prime")
	}
	if b.Fallback.SendBeaconNudge {
		t.Error("SendBeaconNudge should be false — codex has PromptMode=arg, beacon goes via argv")
	}
	if b.Fallback.StartupNudgeDelayMs <= 0 {
		t.Errorf("StartupNudgeDelayMs must be positive for gt prime to settle, got %d", b.Fallback.StartupNudgeDelayMs)
	}
	if b.WorkNudge == "" {
		t.Error("WorkNudge must contain the work-instructions text")
	}
	if !strings.Contains(b.FullPromptForNudge, b.WorkNudge) {
		t.Error("FullPromptForNudge should contain WorkNudge")
	}
}
