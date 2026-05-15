package deacon

// BootstrapPrompt is the initial instruction handed to a freshly spawned Deacon
// session by both `gt deacon start` and the daemon's auto-respawn path.
//
// Architecture (gt-097): the Deacon runs in CRON-SPAWN mode. Each Deacon
// session does exactly ONE patrol cycle and then terminates. The daemon's
// periodic ensureDeaconRunning (driven by the recovery heartbeat tick, ~3 min)
// spawns a fresh Deacon when none is running, which produces a natural patrol
// cadence without requiring the agent to loop in-process. Earlier "perpetual
// loop" attempts (PR #1) repeatedly failed because the agent treated each
// cycle as a discrete task and went idle waiting for respawn — the new model
// matches what the agent already wants to do.
//
// Keep this prompt explicit about (a) one-cycle-then-exit semantics,
// (b) the exit command, and (c) the right way to handle plugin-mail nudges
// that arrive via tmux paste during the cycle.
const BootstrapPrompt = "I am Deacon. CRON-SPAWN MODE: this session does exactly ONE patrol cycle, then exits. The daemon will spawn a fresh Deacon at its next heartbeat tick.\n\nBootstrap: run `gt deacon heartbeat`, then `gt hook`. If empty, run `gt sling mol-deacon-patrol deacon` to create a patrol cycle.\n\nThen `gt prime --hook` for full role context, and follow EVERY step of `mol-deacon-patrol` in DAG order. The final step (`exit-after-cycle`) calls `gt patrol report --summary ... --steps ...` to close the cycle wisp, then `gt deacon stop` to terminate this session. DO NOT call `gt mol step await-signal`. DO NOT start a second cycle in this session.\n\nIgnore plugin-mail nudges that arrive via tmux paste — they do not persist in `gt mail inbox` and are not work. Stay focused on completing this one cycle and exiting."
