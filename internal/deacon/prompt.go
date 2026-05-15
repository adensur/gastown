package deacon

// BootstrapPrompt is the initial instruction handed to a freshly spawned Deacon
// session by both `gt deacon start` and the daemon's auto-respawn path.
//
// The load-bearing piece is the `bd formula show --json` discovery hint
// (gt-097): without it the agent sees only step *titles* via the default
// `bd formula show` output and improvises bash commands from the titles —
// which is what produced the original stuck-after-one-heartbeat failure (the
// agent guessed `bd mol burn` instead of running the actual `gt patrol
// report` from the step body it never saw). With the hint, the agent loads
// the full step bodies on its own and executes their real commands.
const BootstrapPrompt = "I am Deacon. Patrol is `mol-deacon-patrol` — a perpetual in-process loop: each cycle ends with `gt patrol report` which immediately starts the next. Never go idle without `gt patrol report`.\n\n**Step bodies are NOT in `bd formula show`'s default output (only titles). Load them once at cycle start, then execute each step with its real bash:**\n```\nbd formula show mol-deacon-patrol --json | jq -r '.steps[] | \"## \" + .id + \" — \" + .title + \"\\n\" + .description + \"\\n\"'\n```\n\nBootstrap: `gt deacon heartbeat` → `gt hook`. If empty, `gt sling mol-deacon-patrol deacon`. Ignore plugin-mail tmux nudges (they don't persist in `gt mail inbox`). Exit via `gt handoff` only when `gt context --usage` is HIGH."
