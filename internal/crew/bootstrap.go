package crew

import (
	"time"

	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/runtime"
	"github.com/steveyegge/gastown/internal/session"
)

// StartupBootstrap holds the prompt content needed to bring a crew agent to
// life accounting for hook/prompt-capability differences across runtimes.
//
// Claude has a SessionStart hook that auto-runs gt prime, so the bare beacon
// argv is enough. Codex / auggie / amp have no hooks, so they need an
// explicit "Run gt prime" instruction in the beacon plus a delayed nudge
// carrying the work instructions once gt prime has finished settling.
//
// Built once from a RuntimeConfig and consumed by both the fresh-spawn path
// (Manager.Start) and the in-place restart path (cmd.startOrRestartCrewMember).
// Mirrors the bootstrap pattern in internal/polecat/session_manager.go.
type StartupBootstrap struct {
	// Beacon is the formatted startup beacon passed as the agent's initial
	// prompt argument. For non-hook agents it includes a "Run gt prime"
	// instruction so the agent can bootstrap itself without a hook.
	Beacon string

	// WorkNudge is the work-instructions text delivered as a follow-up nudge
	// after gt prime has had time to run. Empty for hook+prompt agents.
	WorkNudge string

	// FullPromptForNudge is the beacon + work instructions concatenated, for
	// promptless agents that must receive the entire startup prompt via nudge
	// rather than as an argv argument.
	FullPromptForNudge string

	// Fallback describes which post-spawn deliveries are required. See
	// runtime.StartupFallbackInfo for the matrix.
	Fallback *runtime.StartupFallbackInfo
}

// BuildStartupBootstrap constructs the prompt content for bringing up a crew
// agent. topic is the beacon topic (e.g. "start", "restart") used for /resume
// session identification.
func BuildStartupBootstrap(rigName, crewName, topic string, rc *config.RuntimeConfig) *StartupBootstrap {
	fallbackInfo := runtime.GetStartupFallbackInfo(rc)
	beaconConfig := session.BeaconConfig{
		Recipient:               session.BeaconRecipient("crew", crewName, rigName),
		Sender:                  "human",
		Topic:                   topic,
		IncludePrimeInstruction: fallbackInfo.IncludePrimeInBeacon,
		ExcludeWorkInstructions: fallbackInfo.SendStartupNudge,
	}
	beacon := session.FormatStartupBeacon(beaconConfig)
	workNudge := runtime.StartupNudgeContent()
	full := session.BuildStartupPrompt(beaconConfig, workNudge)
	return &StartupBootstrap{
		Beacon:             beacon,
		WorkNudge:          workNudge,
		FullPromptForNudge: full,
		Fallback:           fallbackInfo,
	}
}

// bootstrapSession is the tmux surface DeliverStartupFallbacks needs.
// Satisfied by *tmux.Tmux. Kept as an interface so callers can swap mocks
// in tests without dragging the whole tmux package in.
type bootstrapSession interface {
	NudgeSession(sessionID, message string) error
	WaitForRuntimeReady(sessionID string, rc *config.RuntimeConfig, timeout time.Duration) error
}

// DeliverStartupFallbacks runs the post-spawn nudge sequence required by the
// agent's hook/prompt capabilities. No-op for hook+prompt agents (Claude).
// All errors are swallowed: nudge delivery is best-effort, just like the
// polecat path. Caller is responsible for prior dialog acceptance and
// shell→agent transition checks (AcceptStartupDialogs / WaitForCommand).
//
// Sequence:
//  1. Wait for the agent's runtime prompt to appear (ReadyPromptPrefix or
//     ReadyDelayMs).
//  2. If the agent has no prompt support, deliver beacon + work instructions
//     via a single nudge.
//  3. Otherwise, if a non-hook agent, wait StartupNudgeDelayMs more for
//     gt prime to settle, then nudge just the work instructions.
func DeliverStartupFallbacks(t bootstrapSession, sessionID string, b *StartupBootstrap, rc *config.RuntimeConfig, timeout time.Duration) {
	if b == nil || b.Fallback == nil {
		return
	}

	// Step 1: wait for the agent's TUI to be ready at its prompt. Harmless
	// for hook+prompt agents — they just return early and the rest is a no-op.
	_ = t.WaitForRuntimeReady(sessionID, rc, timeout)

	// Step 2: promptless agents (PromptMode="none") — beacon never reached
	// argv, deliver the full beacon + work instructions in one nudge.
	if b.Fallback.SendBeaconNudge {
		_ = runtime.DeliverStartupPromptFallback(t, sessionID, b.FullPromptForNudge, rc, timeout)
		return
	}

	// Step 3: prompt-capable non-hook agents — beacon already in argv. Wait
	// for gt prime to settle, then nudge work instructions.
	if b.Fallback.StartupNudgeDelayMs > 0 {
		primeWaitRC := runtime.RuntimeConfigWithMinDelay(rc, b.Fallback.StartupNudgeDelayMs)
		_ = t.WaitForRuntimeReady(sessionID, primeWaitRC, timeout)
	}
	if b.Fallback.SendStartupNudge {
		_ = t.NudgeSession(sessionID, b.WorkNudge)
	}
}
