package notify

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/nudge"
	"github.com/steveyegge/gastown/internal/session"
	"github.com/steveyegge/gastown/internal/tmux"
)

type Mode string

const (
	ModeImmediate Mode = "immediate"
	ModeQueue     Mode = "queue"
	ModeWaitIdle  Mode = "wait-idle"
)

type DirectFormat int

const (
	DirectPlain DirectFormat = iota
	DirectQueuedReminder
)

type Request struct {
	Tmux        *tmux.Tmux
	TownRoot    string
	SessionName string
	Message     string
	Sender      string
	Priority    string
	Kind        string
	ThreadID    string
	Severity    string

	Mode          Mode
	DirectFormat  DirectFormat
	WaitTimeout   time.Duration
	WatchTimeout  time.Duration
	WatchInterval time.Duration

	StartPoller bool
	Stderr      io.Writer
}

type Result struct {
	Direct  bool
	Queued  bool
	Drained int
}

func Deliver(req Request) (Result, error) {
	if err := req.validate(); err != nil {
		return Result{}, err
	}
	if logged, err := maybeLogTestDelivery(req); logged || err != nil {
		return Result{Direct: logged}, err
	}

	mode := req.Mode
	if mode == "" {
		mode = ModeWaitIdle
	}

	switch mode {
	case ModeQueue:
		if err := enqueue(req); err != nil {
			return Result{}, err
		}
		return Result{Queued: true}, nil

	case ModeWaitIdle:
		return deliverWaitIdle(req)

	case ModeImmediate:
		if err := direct(req); err != nil {
			return Result{}, err
		}
		return Result{Direct: true}, nil

	default:
		return Result{}, fmt.Errorf("invalid delivery mode %q", mode)
	}
}

type WatchRequest struct {
	Tmux         *tmux.Tmux
	TownRoot     string
	SessionName  string
	Timeout      time.Duration
	PollInterval time.Duration
	Stderr       io.Writer
}

func WatchAndDeliverQueued(req WatchRequest) {
	stderr := req.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	pollInterval := req.PollInterval
	if pollInterval <= 0 {
		pollInterval = time.Second
	}
	fmt.Fprintf(stderr, "Watching %s for idle (up to %s)...\n", req.SessionName, req.Timeout)
	deadline := time.Now().Add(req.Timeout)
	for time.Now().Before(deadline) {
		time.Sleep(pollInterval)

		if nudge.QueueLen(req.TownRoot, req.SessionName) == 0 {
			return
		}
		if req.Tmux == nil {
			fmt.Fprintf(stderr, "idle-watcher: tmux client missing for %s\n", req.SessionName)
			return
		}
		if exists, _ := req.Tmux.HasSession(req.SessionName); !exists {
			return
		}
		result, err := DrainAndDeliverQueued(DrainRequest{
			Tmux:               req.Tmux,
			TownRoot:           req.TownRoot,
			SessionName:        req.SessionName,
			IdleTimeout:        pollInterval,
			HasPromptDetection: true,
			Stderr:             stderr,
		})
		if err != nil {
			fmt.Fprintf(stderr, "idle-watcher: delivery for %s failed: %v\n", req.SessionName, err)
			return
		}
		if result.Direct || result.Drained > 0 {
			return
		}
	}
}

type DrainRequest struct {
	Tmux               *tmux.Tmux
	TownRoot           string
	SessionName        string
	IdleTimeout        time.Duration
	HasPromptDetection bool
	SkipEscape         bool
	Stderr             io.Writer
}

func DrainAndDeliverQueued(req DrainRequest) (Result, error) {
	if req.Tmux == nil {
		return Result{}, errors.New("tmux client is required")
	}
	waitOpts := WaitOpts(req.SessionName)
	waitErr := req.Tmux.WaitForIdleWithOpts(req.SessionName, req.IdleTimeout, waitOpts)
	if ShouldSkipDrainUntilIdle(req.HasPromptDetection || waitOpts.RequireEmptyInput, waitErr) {
		return Result{}, nil
	}

	drained, err := nudge.Drain(req.TownRoot, req.SessionName)
	if err != nil {
		return Result{}, err
	}
	if len(drained) == 0 {
		return Result{}, nil
	}

	directReq := Request{
		Tmux:         req.Tmux,
		TownRoot:     req.TownRoot,
		SessionName:  req.SessionName,
		Message:      nudge.FormatForInjection(drained),
		Mode:         ModeImmediate,
		DirectFormat: DirectPlain,
		Stderr:       req.Stderr,
	}
	if err := directWithSkipEscape(directReq, req.SkipEscape); err != nil {
		return Result{Drained: len(drained)}, err
	}
	return Result{Direct: true, Drained: len(drained)}, nil
}

func ShouldSkipDrainUntilIdle(hasPromptDetection bool, waitErr error) bool {
	return hasPromptDetection && waitErr != nil
}

func WaitOpts(sessionName string) tmux.WaitForIdleOpts {
	return tmux.WaitForIdleOpts{RequireEmptyInput: session.IsHumanFacingSession(sessionName)}
}

func deliverWaitIdle(req Request) (Result, error) {
	if req.TownRoot == "" {
		return Result{}, fmt.Errorf("--mode=wait-idle requires a Gas Town workspace")
	}

	if agentName, ok := promptlessAgent(req); ok {
		fmt.Fprintf(stderr(req), "wait-idle: %s agent %q has no prompt detection, using queue mode\n", req.SessionName, agentName)
		if err := enqueue(req); err != nil {
			if session.IsHumanFacingSession(req.SessionName) {
				return Result{}, fmt.Errorf("queue fallback failed for human-facing session %q; refusing direct delivery: %w", req.SessionName, err)
			}
			if directErr := directQueued(req); directErr != nil {
				return Result{}, directErr
			}
			return Result{Direct: true}, nil
		}
		if req.StartPoller {
			if _, err := nudge.StartPoller(req.TownRoot, req.SessionName); err != nil {
				fmt.Fprintf(stderr(req), "wait-idle: could not start nudge poller for %s: %v\n", req.SessionName, err)
			}
		}
		return Result{Queued: true}, nil
	}

	waitErr := req.Tmux.WaitForIdleWithOpts(req.SessionName, req.WaitTimeout, WaitOpts(req.SessionName))
	if waitErr == nil {
		if err := directQueued(req); err != nil {
			return Result{}, err
		}
		return Result{Direct: true}, nil
	}
	if errors.Is(waitErr, tmux.ErrSessionNotFound) || errors.Is(waitErr, tmux.ErrNoServer) {
		return Result{}, fmt.Errorf("wait-idle: %w", waitErr)
	}

	if err := enqueue(req); err != nil {
		if session.IsHumanFacingSession(req.SessionName) {
			return Result{}, fmt.Errorf("queue fallback failed for human-facing session %q; refusing direct delivery: %w", req.SessionName, err)
		}
		fmt.Fprintf(stderr(req), "Warning: queue fallback failed (%v), delivering immediately\n", err)
		if directErr := directQueued(req); directErr != nil {
			return Result{}, directErr
		}
		return Result{Direct: true}, nil
	}
	if req.WatchTimeout > 0 {
		WatchAndDeliverQueued(WatchRequest{
			Tmux:         req.Tmux,
			TownRoot:     req.TownRoot,
			SessionName:  req.SessionName,
			Timeout:      req.WatchTimeout,
			PollInterval: req.WatchInterval,
			Stderr:       req.Stderr,
		})
	}
	return Result{Queued: true}, nil
}

func directQueued(req Request) error {
	if req.DirectFormat == DirectQueuedReminder {
		req.Message = nudge.FormatForInjection([]nudge.QueuedNudge{req.queuedNudge()})
		req.DirectFormat = DirectPlain
	}
	return direct(req)
}

func direct(req Request) error {
	return directWithSkipEscape(req, false)
}

func directWithSkipEscape(req Request, skipEscape bool) error {
	opts := tmux.NudgeOpts{
		TownRoot:   req.TownRoot,
		SkipEscape: skipEscape || escapeCancelsRequest(req),
	}
	return req.Tmux.NudgeSessionWithOpts(req.SessionName, req.Message, opts)
}

func enqueue(req Request) error {
	if req.TownRoot == "" {
		return fmt.Errorf("--mode=queue requires a Gas Town workspace")
	}
	return nudge.Enqueue(req.TownRoot, req.SessionName, req.queuedNudge())
}

func (req Request) queuedNudge() nudge.QueuedNudge {
	priority := req.Priority
	if priority == "" {
		priority = nudge.PriorityNormal
	}
	return nudge.QueuedNudge{
		Sender:   req.Sender,
		Message:  req.Message,
		Priority: priority,
		Kind:     req.Kind,
		ThreadID: req.ThreadID,
		Severity: req.Severity,
	}
}

func (req Request) validate() error {
	if req.Tmux == nil {
		return errors.New("tmux client is required")
	}
	if req.SessionName == "" {
		return errors.New("session name is required")
	}
	return nil
}

func maybeLogTestDelivery(req Request) (bool, error) {
	logPath := os.Getenv("GT_TEST_NUDGE_LOG")
	if logPath == "" {
		return false, nil
	}
	entry := fmt.Sprintf("nudge:%s:%s:%s\n", req.SessionName, req.Sender, req.Message)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false, err
	}
	_, writeErr := f.WriteString(entry)
	closeErr := f.Close()
	if writeErr != nil {
		return false, writeErr
	}
	if closeErr != nil {
		return false, closeErr
	}
	return true, nil
}

func promptlessAgent(req Request) (string, bool) {
	agentName, err := req.Tmux.GetEnvironment(req.SessionName, "GT_AGENT")
	if err != nil || agentName == "" {
		return "", false
	}
	preset := config.GetAgentPresetByName(agentName)
	return agentName, preset != nil && preset.ReadyPromptPrefix == ""
}

func escapeCancelsRequest(req Request) bool {
	agentName, err := req.Tmux.GetEnvironment(req.SessionName, "GT_AGENT")
	if err != nil || agentName == "" {
		return false
	}
	preset := config.GetAgentPresetByName(agentName)
	return preset != nil && preset.EscapeCancelsRequest
}

func stderr(req Request) io.Writer {
	if req.Stderr != nil {
		return req.Stderr
	}
	return os.Stderr
}
