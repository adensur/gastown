package session

import "strings"

// IsHumanFacingAddress reports whether an address points at a session a human
// typically types into. Direct tmux delivery to these targets must require an
// empty input box to avoid splicing notification text into in-progress input.
func IsHumanFacingAddress(address string) bool {
	switch strings.TrimSuffix(address, "/") {
	case "mayor":
		return true
	}
	return false
}

// IsHumanFacingSession reports whether a tmux session is human-facing.
func IsHumanFacingSession(sessionName string) bool {
	switch sessionName {
	case MayorSessionName(), "gt-mayor":
		return true
	}
	return false
}
