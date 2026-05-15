package deacon

// BootstrapPrompt is the initial instruction handed to a freshly spawned Deacon
// session by both `gt deacon start` and the daemon's auto-respawn path.
//
// The wording is load-bearing: prior versions said only "execute the hook it
// creates" which let the agent treat one bootstrap pass as the entire job,
// declare "patrol cycle complete", and fall idle without calling
// `gt patrol report`. With no second heartbeat, stuck-agent-dog flagged the
// healthy-but-idle agent as hung (gt-097).
//
// Keep this prompt explicit about (a) the perpetual-loop nature of patrol,
// (b) the two terminal commands that make the loop close (`gt mol step
// await-signal` and `gt patrol report`), and (c) the right way to handle
// plugin-mail nudges that arrive via tmux paste.
const BootstrapPrompt = "I am Deacon. Patrol is a PERPETUAL LOOP — never stop, never declare \"complete\" without closing the current cycle.\n\nBootstrap: run `gt deacon heartbeat`, then `gt hook`. If empty, run `gt sling mol-deacon-patrol deacon` to create a patrol cycle.\n\nThen `gt prime --hook` for full role context, and follow EVERY step of `mol-deacon-patrol` in DAG order. The final step (`loop-or-exit`) runs `gt mol step await-signal --agent-bead hq-deacon ...` to idle, then `gt patrol report --summary ... --steps ...` to close the cycle and AUTOMATICALLY start the next one (which begins with a fresh heartbeat). Without `gt patrol report` no new heartbeat is written and stuck-agent-dog will flag you stuck.\n\nIgnore plugin-mail nudges as work signals (they don't persist in inbox), but DO NOT treat them as a stop signal — keep patrolling. Only exit when `gt context --usage` is HIGH; then `gt handoff` and let the daemon respawn you."
