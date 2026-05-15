package cmd

import (
	"fmt"
	"strings"

	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/crew"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/workspace"
)

// DelayedCrewInfo describes a crew session that should be started only after
// sling has attached work to the crew member's hook.
type DelayedCrewInfo struct {
	RigName     string
	CrewName    string
	AgentID     string
	SessionName string
	WorkDir     string

	startAfterHook func(beforeStart func() error) (string, error)
}

func (d *DelayedCrewInfo) StartAfterHook(beforeStart func() error) (string, error) {
	if d == nil || d.startAfterHook == nil {
		return "", fmt.Errorf("delayed crew start not configured")
	}
	return d.startAfterHook(beforeStart)
}

func parseCrewSlingTarget(target string) (rigName, crewName string, ok bool) {
	parts := strings.Split(target, "/")
	if len(parts) != 3 || !strings.EqualFold(parts[1], "crew") {
		return "", "", false
	}
	if parts[0] == "" || parts[2] == "" {
		return "", "", false
	}
	return parts[0], parts[2], true
}

func resolveDelayedCrewTarget(target string, opts ResolveTargetOptions) (*ResolvedTarget, bool, error) {
	rigName, crewName, ok := parseCrewSlingTarget(target)
	if !ok {
		return nil, false, nil
	}

	agentID := fmt.Sprintf("%s/crew/%s", rigName, crewName)
	if opts.DryRun {
		fmt.Printf("Would start crew worker '%s' in rig '%s' after hook write\n", crewName, rigName)
		return &ResolvedTarget{
			Agent: agentID,
			Pane:  "<new-crew-pane>",
			DelayedCrewInfo: &DelayedCrewInfo{
				RigName:     rigName,
				CrewName:    crewName,
				AgentID:     agentID,
				SessionName: crewSessionName(rigName, crewName),
			},
		}, true, nil
	}

	townRoot := opts.TownRoot
	if townRoot == "" {
		townRoot, _ = workspace.FindFromCwd()
	}
	if townRoot != "" {
		if blocked, reason := IsRigParkedOrDocked(townRoot, rigName); blocked {
			undoCmd := "gt rig unpark"
			if reason == "docked" {
				undoCmd = "gt rig undock"
			}
			return nil, true, fmt.Errorf("cannot sling to %s rig %q\n%s %s", reason, rigName, undoCmd, rigName)
		}
	}

	crewMgr, r, err := getCrewManager(rigName)
	if err != nil {
		return nil, true, fmt.Errorf("preparing crew target: %w", err)
	}

	sessionName := crewMgr.SessionName(crewName)
	info := &DelayedCrewInfo{
		RigName:     rigName,
		CrewName:    crewName,
		AgentID:     agentID,
		SessionName: sessionName,
		WorkDir:     r.Path,
	}
	info.startAfterHook = func(beforeStart func() error) (string, error) {
		startOpts := crew.StartOptions{
			Account:       opts.Account,
			AgentOverride: opts.Agent,
			Topic:         "assigned",
			BeforeStart: func(_ *crew.CrewWorker) error {
				bd := beads.New(beads.ResolveBeadsDir(r.Path))
				if _, err := upsertCrewAgentBead(bd, townRoot, rigName, crewName); err != nil {
					return fmt.Errorf("upserting crew agent bead: %w", err)
				}
				if beforeStart != nil {
					return beforeStart()
				}
				return nil
			},
		}
		if err := crewMgr.Start(crewName, startOpts); err != nil {
			return "", err
		}
		pane, err := getSessionPane(sessionName)
		if err != nil {
			fmt.Printf("%s Could not resolve pane for started crew %s: %v\n", style.Dim.Render("Warning:"), crewName, err)
			return "", nil
		}
		return pane, nil
	}

	return &ResolvedTarget{
		Agent:           agentID,
		WorkDir:         r.Path,
		DelayedCrewInfo: info,
	}, true, nil
}

func rollbackDelayedCrewHook(beadID, hookWorkDir string) {
	if beadID == "" {
		return
	}
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		fmt.Printf("  %s Could not find workspace to rollback bead %s: %v\n", style.Dim.Render("Warning:"), beadID, err)
		return
	}

	if info, infoErr := getBeadInfoForRollback(beadID); infoErr == nil {
		existingMolecules := collectExistingMoleculesForRollback(info)
		if len(existingMolecules) > 0 {
			if burnErr := burnExistingMoleculesForRollback(existingMolecules, beadID, townRoot); burnErr != nil {
				fmt.Printf("  %s Could not burn stale molecule(s) from %s: %v\n", style.Dim.Render("Warning:"), beadID, burnErr)
			} else {
				fmt.Printf("  %s Burned %d stale molecule(s): %s\n",
					style.Dim.Render("○"), len(existingMolecules), strings.Join(existingMolecules, ", "))
			}
		}
	} else {
		fmt.Printf("  %s Could not inspect bead %s for stale molecules: %v\n", style.Dim.Render("Warning:"), beadID, infoErr)
	}

	unhookDir := beads.ResolveHookDir(townRoot, beadID, hookWorkDir)
	if err := BdCmd("update", beadID, "--status=open", "--assignee=").Dir(unhookDir).Run(); err != nil {
		fmt.Printf("  %s Could not unhook bead %s: %v\n", style.Dim.Render("Warning:"), beadID, err)
	} else {
		fmt.Printf("  %s Unhooked bead %s\n", style.Dim.Render("○"), beadID)
	}
}
