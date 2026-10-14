# STORY_LOG

Task: Optional random computer opponent for browser tic-tac-toe (SPEC v3, PLAN.md)
Frontier date: 2026-10-14

## Quality gates (run from /workspace after implementation)

All four declared gates pass on the final candidate:

- `bash /inputs/resources/team/scripts/build.sh` → PASS (build products in /runtime/build, application source untouched)
- `go test ./...` → PASS (fresh `-count=1` run: `ok tic-tac-toe`)
- `go vet ./...` → PASS
- `bash /inputs/resources/team/scripts/lint.sh` → PASS (golangci-lint v2, team config: 0 issues)

Race detection (`go test -race`) could not be run because the workload image has no C compiler
(`gcc`), so CGO is unavailable; the race tool is not one of the four declared gates. All new
fixture tests drive sequential HTTP requests through the production handlers, and the
`pickComputerMove` seam is only assigned between requests, never concurrently.

## Story log

DONE US-001 — Server mode state, lifecycle, and reset semantics (AC-1, AC-2, AC-3, AC-6, AC-7)
  Evidence: `mode_test.go` + `server.go`. `newGameState` defaults to `mode: "two-player"` with an
  empty board, X to move, no result; every state response carries the additive `Mode` field.
  `POST /mode` starts a fresh game for a different valid mode (mid-game and after a finished
  game), is a no-op 200 for the already-active mode, and 400 `{"error":"invalid mode"}` (state
  untouched, verified by bit-for-bit GET /state comparison) for any other body; non-POST is 405.
  `POST /reset` clears the board/result and keeps the active mode in both modes and after game
  over. Two-player `/move` alternation is unchanged (existing `server_test.go` passes untouched;
  `TestTwoPlayer_NeverComputes` proves no automatic O in two-player mode).

DONE US-002 — Computer move after each valid nonterminal human move (AC-4, AC-8)
  Evidence: `computer_test.go` + `server.go`. In computer mode the same `/move` request places
  the human X and then exactly one O via `pickComputerMove` (default: `math/rand.Intn` over empty
  cells — stdlib only, AC-8) and returns X to move. Fixture A (50 runs over the real random
  source): every run is legal, exactly one O in an allowed empty cell, no particular random
  outcome asserted. Fixture B through the real handlers with the seam: pick 5 → `gameOver=true,
  winner="O"` (computer win, per the approved plan-review clarification AC-4); pick 6 →
  nonterminal pair, X to move, not over. Equal-counts invariant asserted (`|X|==|O|` at X's turn)
  on every reachable pre-move fixture board.

DONE US-003 — No computer move after invalid, winning, or draw-filling human moves (AC-5)
  Evidence: `computer_test.go`. Fixture C: human X win → exactly 3 X / 2 O, gameOver true, winner
  X, no computer O. Fixture D: human fills the board → draw, exactly 5 X / 4 O, no computer O.
  Fixture E: occupied cell, out-of-range cell (same `cell 99 is out of range (0-8)` message), and
  malformed JSON (same `invalid JSON body` status/message) each 400 in computer mode with a live O
  threat and leave the state bit-for-bit unchanged. The computer-fill (draw) branch is
  unreachable under the equal-counts invariant (the human always makes the final move) and is
  deliberately not chased; the computer-win terminal branch is deterministically covered.

DONE US-004 — Browser mode selector and mode display (AC-1, AC-7)
  Evidence: `index.html`, `style.css`, `mode_test.go` static-asset checks. The page offers exactly
  "Two players" / "Computer" radio controls plus a mode display, styled only through the separate
  `style.css` (no inline `<style>`), with the existing board/status/reset markup and IDs intact.
  The script renders `state.mode` on initial load, after moves/reset/mode change, posts
  `{"mode": ...}` to `POST /mode` on change, treats the already-active mode as a no-op, and
  restores mode + game state from GET /state on refresh. A `moveInFlight` guard suppresses
  duplicate submits while a paired `/move` is pending. Served-HTML assertions are static-asset
  regression checks only and are not claimed to establish browser behavior.

DONE US-005 — Regression sweep and final gate run (AC-6)
  Evidence: `main.go` and `main_test.go` are byte-identical to the pre-task baseline (sha256
  verified), so the CLI game, terminal-result handling, and board logic are untouched; the full
  existing suite (CLI/board/server + asset tests) plus all new mode/computer tests pass. The four
  declared gates pass one final time from /workspace; lint findings were zero and no lint
  configuration was weakened. No `manual-browser-checklist.md` artifact is produced: the approved
  plan-review feedback confirmed the checklist below already lives in PLAN.md and that the pinned
  build manifest publishes STORY_LOG.md only. No Go/HTML test claims browser validation.

## Manual real-browser checklist (executed independently by the delegated coordinator)

Plan: `go run . -addr=localhost:8080` from /workspace, then in a normal browser tab:

1. **Initial state / mode choice (AC-1, AC-7):** opening the page shows the mode selector with
   exactly "Two players" and "Computer", mode display "Two players", an empty board, X to move.
2. **Mode selection (AC-2):** click "Computer". An empty board appears, X to move, no result, and
   the mode display shows "Computer".
3. **Paired move (AC-4, AC-8):** click an empty square. In the same user action the human X
   appears, then one O appears on a random empty square; afterwards it is X's turn and rapid
   double-clicks do not submit duplicate moves (in-flight guard).
4. **Reset (AC-3):** click Reset Game. Board clears, X to move, no result, mode stays "Computer".
5. **Refresh (AC-2, AC-7):** reload the page. Mode stays "Computer" and the game continues from
   the server state (board, turn, result reappear); refresh does not start a new game or switch
   modes.

## Plan-review clarification incorporated (from the approved workflow feedback)

US-002's "gameOver=false" applies only to a nonterminal HUMAN+COMPUTER pair; a computer O win
must return `gameOver=true` as explicitly required by fixture B and SPEC AC-4. The server sets
`gameOver=true, winner="O"` after a winning computer move (covered deterministically by fixture B
with the seam), and `gameOver=false` only for nonterminal pairs. No browser validation is claimed
by Go/HTML tests; real-browser checks are the coordinator's job per the checklist above.