package cmd

import (
	"errors"
	"testing"
)

func TestParseCrewSlingTarget(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		wantRig  string
		wantCrew string
		wantOK   bool
	}{
		{
			name:     "crew path",
			target:   "gastown/crew/toecutter",
			wantRig:  "gastown",
			wantCrew: "toecutter",
			wantOK:   true,
		},
		{
			name:     "case insensitive role segment",
			target:   "gastown/Crew/toecutter",
			wantRig:  "gastown",
			wantCrew: "toecutter",
			wantOK:   true,
		},
		{
			name:   "polecat path",
			target: "gastown/polecats/toecutter",
		},
		{
			name:   "missing crew name",
			target: "gastown/crew",
		},
		{
			name:   "too many segments",
			target: "gastown/crew/toecutter/extra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRig, gotCrew, gotOK := parseCrewSlingTarget(tt.target)
			if gotOK != tt.wantOK {
				t.Fatalf("parseCrewSlingTarget(%q) ok = %v, want %v", tt.target, gotOK, tt.wantOK)
			}
			if gotRig != tt.wantRig || gotCrew != tt.wantCrew {
				t.Fatalf("parseCrewSlingTarget(%q) = (%q, %q), want (%q, %q)",
					tt.target, gotRig, gotCrew, tt.wantRig, tt.wantCrew)
			}
		})
	}
}

func TestResolveTargetReturnsDelayedCrewWhenSessionIsInactive(t *testing.T) {
	prevResolve := resolveTargetAgentFn
	t.Cleanup(func() { resolveTargetAgentFn = prevResolve })
	resolveTargetAgentFn = func(target string) (string, string, string, error) {
		return "", "", "", errors.New("no active session")
	}

	resolved, err := resolveTarget("gastown/crew/toecutter", ResolveTargetOptions{DryRun: true})
	if err != nil {
		t.Fatalf("resolveTarget: %v", err)
	}
	if resolved.Agent != "gastown/crew/toecutter" {
		t.Fatalf("Agent = %q, want gastown/crew/toecutter", resolved.Agent)
	}
	if resolved.Pane != "<new-crew-pane>" {
		t.Fatalf("Pane = %q, want <new-crew-pane>", resolved.Pane)
	}
	if resolved.DelayedCrewInfo == nil {
		t.Fatalf("DelayedCrewInfo is nil")
	}
	if resolved.DelayedCrewInfo.SessionName == "" {
		t.Fatalf("DelayedCrewInfo.SessionName is empty")
	}
}

func TestResolveTargetKeepsRunningCrewSessionPath(t *testing.T) {
	prevResolve := resolveTargetAgentFn
	t.Cleanup(func() { resolveTargetAgentFn = prevResolve })
	resolveTargetAgentFn = func(target string) (string, string, string, error) {
		return "gastown/crew/toecutter", "%1", "/tmp/toecutter", nil
	}

	resolved, err := resolveTarget("gastown/crew/toecutter", ResolveTargetOptions{})
	if err != nil {
		t.Fatalf("resolveTarget: %v", err)
	}
	if resolved.DelayedCrewInfo != nil {
		t.Fatalf("DelayedCrewInfo = %+v, want nil for running crew", resolved.DelayedCrewInfo)
	}
	if resolved.Agent != "gastown/crew/toecutter" || resolved.Pane != "%1" || resolved.WorkDir != "/tmp/toecutter" {
		t.Fatalf("resolved = %+v, want running crew target", resolved)
	}
}
