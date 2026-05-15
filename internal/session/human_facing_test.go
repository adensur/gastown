package session

import "testing"

func TestIsHumanFacingAddress(t *testing.T) {
	tests := []struct {
		address string
		want    bool
	}{
		{"mayor", true},
		{"mayor/", true},
		{"deacon", false},
		{"overseer", false},
		{"gastown/crew/Dementus", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			if got := IsHumanFacingAddress(tt.address); got != tt.want {
				t.Errorf("IsHumanFacingAddress(%q) = %v, want %v", tt.address, got, tt.want)
			}
		})
	}
}

func TestIsHumanFacingSession(t *testing.T) {
	tests := []struct {
		sessionName string
		want        bool
	}{
		{MayorSessionName(), true},
		{"gt-mayor", true},
		{DeaconSessionName(), false},
		{OverseerSessionName(), false},
		{"gt-crew-Dementus", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.sessionName, func(t *testing.T) {
			if got := IsHumanFacingSession(tt.sessionName); got != tt.want {
				t.Errorf("IsHumanFacingSession(%q) = %v, want %v", tt.sessionName, got, tt.want)
			}
		})
	}
}
