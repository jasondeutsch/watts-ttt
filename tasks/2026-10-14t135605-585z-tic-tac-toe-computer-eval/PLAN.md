# PLAN: Optional random computer opponent for browser tic-tac-toe

Task documents: /runtime/task
Implementation root: /workspace (the application workspace supplied by Watts)

Application source, tests, and build files belong in /workspace and its normal source directories. The task directory stores workflow documents, story logs, review artifacts, and execution evidence only. No application code or module is created under /runtime/task. Quality gates run from /workspace, the Go module root.
Spec ref: SPEC.md v3
Risk tier at Planning: 1, from SPEC.md

*The planning agent drafts this file. The plan-review workflow gate records human approval before implementation starts.*

## Architecture and approach

Extend the existing single-server game state with a mode concept and a computer-move step, keeping the current architecture (one server-wide `gameState` guarded by a mutex, one `main` package, embedded static assets) untouched in shape. The diff stays in the five files listed under Systems touched; `main.go`, the CLI, `go.mod`, and the existing API error behavior do not change.

Server side (`server.go`):

- `gameState` gains two unexported fields:
  - `mode string` — `"two-player"` (initial default, AC-1) or `"computer"`. Stored per game state, never persisted.
  - `pickComputerMove func(b board) int` — a test seam. `newGameState` defaults it to a uniform-random choice over the currently empty cells using the Go standard library only (`math/rand` `rand.Intn` over the list of empty cells). The HTTP handler calls this field exactly as production runs it; tests (same `package main`) may replace it with a deterministic selector to force particular outcomes. The seam is unexported, so there is no exported API or CLI change.
- `stateResponse` gains `Mode string` (`json:"mode"`, values `"two-player"`/`"computer"`), so GET /state identifies the active mode (AC-7) and a page refresh re-reads it from the server (no localStorage: mode lives in server state only).
- New handler `POST /mode` with body `{"mode":"two-player"|"computer"}`:
  - A mode string equal to the current mode: no effect on the game; respond 200 with the current state (spec assumption: selecting the active mode does nothing).
  - A different valid mode: start a fresh game in the selected mode — empty board, X to move, no result; respond 200 with that state (AC-2).
  - Anything else: 400 with `{"error": "invalid mode"}`; board, turn, result, and mode unchanged.
  - Non-POST: 405 `{"error": "method not allowed"}` (matches existing pattern).
- `/move` keeps every existing validation and error exactly as today (malformed JSON, missing/out-of-range cell, occupied cell, `gameOver` → identical 400 responses). After those checks the handler places the human's X mark and runs the win/draw checks as today. Then, only if `mode == "computer"` and the game is not over, it places exactly one computer O via `pickComputerMove`, runs the win/draw checks again, and leaves `currentPlayer` at X (the human's next turn, AC-4). In two-player mode the existing toggle to X/O runs unchanged, so two-player behavior is byte-identical to today (AC-6). Because the computer O is placed inside the same `/move` request under the same mutex hold, no interleaved request can observe a half-applied pair; an invalid, winning, or draw-filling human move returns before the computer step, so no computer move follows it (AC-5).
- `/reset` clears the board, removes any result, sets the turn to X, and now preserves the active `mode` unchanged (AC-3). It responds 200 with the (same-shaped) state response including the mode.

Client side:

- `index.html` gains a small mode selector presented as "Two players" / "Computer" (radio-style control above the board, below the status line) and a mode display line. The browser script reads `state.mode` on every render (initial load, after every move, after reset), posts `{"mode": ...}` to `/mode` when the selection changes, and re-renders from the returned fresh state. The existing board, status, reset, and fetch structure stays; a lightweight in-flight guard prevents duplicate submits while a paired `/move` (human X + computer O) is pending.
- `style.css` styles the new selector group in the existing look. Styles stay in the separate stylesheet; no inline `<style>` (existing test guards this).
- No JavaScript browser-testing harness is added at all: no headless browser, no `node:vm` DOM/fetch simulator, no `smoke_test.mjs`. Browser behavior is verified by the short real-browser manual checklist in Testing strategy, executed independently by the delegated coordinator. Served-HTML string assertions already present in `server_test.go` are static-asset regression checks only and are not claimed to establish browser behavior.

## Data model and interface changes

- `gameState` (internal): new unexported `mode string`; new unexported `pickComputerMove func(b board) int` (default: uniform random via `math/rand.Intn` over empty cells; stdlib only, no strategy/difficulty/external services — AC-8).
- `stateResponse` (API surface): additive field `Mode string` with `json:"mode"`, `"two-player"` or `"computer"`. All existing fields (`board`, `currentPlayer`, `winner`, `gameOver`) and their semantics are unchanged; existing serializers keep byte-identical output for those fields.
- New endpoint `POST /mode`: request `{"mode":"two-player"|"computer"}`, responses as described above. `GET /state`, `POST /move`, `POST /reset`, `/`, `/style.css` contracts (status codes, error bodies, content types) are unchanged apart from the additive Mode field in state responses.
- No persistence, no schema, no new packages or modules, no exported API or CLI changes.

## Systems touched

Cross-checked against the risk-tiering skill, not from memory.

- Touches:
  - /workspace/server.go — `gameState` fields, `newGameState` default, `stateResponse`, `POST /mode`, `/move` computer branch, `/reset` mode preservation.
  - /workspace/index.html — mode selector, mode display, script wiring to `/mode` and `state.mode`.
  - /workspace/style.css — styling for the selector control (separate stylesheet only).
  - /workspace/server_test.go (or a sibling `_test.go` in the same package) — new tests through the production handlers.
- Does not touch:
  - /workspace/main.go — CLI game, terminal-result handling, board logic, flags.
  - /workspace/main_test.go — unchanged; keeps passing as the CLI/board regression suite.
  - go.mod, go.sum — no new dependencies (stdlib `math/rand` only).
  - External services, network exposure, credentials, entitlements, identity, or access-control logic. Server continues to bind whatever `-addr` main.go passes; default behavior unchanged.

## Technical decisions and alternatives considered

- **Computer move inside the same `/move` request** (chosen) vs. a separate endpoint after the human move or client-driven orchestration (rejected). Placing O in the same request under the same mutex hold makes "exactly one O per valid nonterminal human move" atomic, matches the spec's testability requirement ("tests cannot exercise client-side JavaScript"), and adds no client state machine.
- **Injectable `pickComputerMove` seam** (chosen) vs. statistical many-run tests that observe terminal outcomes under real randomness (rejected). A many-run test must either flake or assert aggregate frequencies to prove the computer-win branch; the unexported seam lets tests force a legal pick and deterministically exercise the exact production terminal branch through the real HTTP handlers while production keeps uniform `math/rand`. No test asserts a particular random outcome, satisfying AC-8.
- **`math/rand.Intn` over the empty-cell list** (chosen) vs. `math/rand/v2`/`rand.N` (rejected: `rand.N` is not available in the stdlib for the Go 1.21 module in go.mod) and vs. a custom shuffler or `crypto/rand` (rejected: overkill and not required; AC-8 asks only for simple stdlib randomness, and this is not security-relevant).
- **Dedicated `POST /mode` endpoint** (chosen) vs. extending `POST /move` with a mode field or a query param (rejected): keeps the `/move` contract and existing tests untouched, treats mode selection as a separate lifecycle action, and matches the existing JSON-body + error-shape conventions.
- **Server-side mode only, no localStorage** (chosen) vs. persisting mode client-side (rejected): the spec defines mode as part of the server's single game state and refresh-retention is satisfied by reading `state.mode` back from GET /state.
- **Real-browser manual checklist for UI verification** (chosen) vs. a headless browser or a `node:vm` DOM/fetch simulator (rejected). The feedback confirmed no new browser dependency is required. Served-HTML string assertions already in the suite are static-asset regression checks; they do not and will not be claimed to establish browser behavior. The delegated coordinator executes the checklist as independent eval verification.
- **Two-button/radio selector above the board** (chosen) vs. a dropdown (rejected: two discrete modes, a radio group is more visible and simpler). Placement and styling were left to the plan by the spec.

## Testing strategy

All new behavior is tested through the production handlers: `httptest.NewServer(newServerMux(gs))` plus the existing helpers in `server_test.go` (`makeMove`, `getState`, `postReset`, `parseStateResponse`). Tests are white-box only where needed (setting the unexported `pickComputerMove` and `mode` fields in `package main`); every submitted request flows through the real mux, real validations, and real `/move`/`/mode`/`/reset` handlers. No `smoke_test.mjs`, no `node:vm` DOM/fetch simulator, no new browser dependency.

Reachable-fixture invariant (stated once, used everywhere): in computer mode the human is always X and moves first, so X to move implies equal X and O counts (`|X| == |O|`) on every reachable pre-move board. After a valid human X move the counts are `|X| = n+1, |O| = n`; after the paired nonterminal computer O move they are `n+1 / n+1` with X to move. Any fixture with unequal counts at X's turn is unreachable and absent from this plan. A consequence: the human always makes the odd-numbered moves and the final filling move, so the computer can never fill the board — the computer-terminal path that is reachable is the computer win (completing an O line), and that is the branch the tests force. The server still runs the win/draw check after the computer move (same code path as today), and draw semantics themselves are covered by existing tests and fixture D.

| Fixture | Pre-move occupancy (X to move) | Submitted human move | Expected result |
|---|---|---|---|
| A (basic paired move) | X={0}, O={3} (1 vs 1) | cell 1 | Not terminal: board X={0,1} plus exactly one new O mark in one of {2,4,5,6,7,8}; currentPlayer X; not game over. Run repeatedly over the real random source: every run legal, never asserts a specific O cell. |
| B (computer terminal/nonterminal) | X={0,2}, O={3,4} (2 vs 2) | cell 8 (nonterminal; empties {1,5,6,7}) | With the seam forcing pick 5: O={3,4,5} wins → gameOver true, winner O. With the seam forcing pick 6: O={3,4,6}, currentPlayer X, not game over. Same production branch, deterministic coverage of both outcomes. |
| C (human win, no computer move) | X={0,1}, O={3,4} (2 vs 2) | cell 2 | Human X wins (row 0-1-2); board has exactly 3 X and 2 O marks — O count remains 2; no computer O placed; gameOver true, winner X. |
| D (human fills board, no computer move) | X={0,2,5,6}, O={1,3,4,8} (4 vs 4) | cell 7 | Human X fills the board; draw; exactly 5 X and 4 O marks; no computer O placed; gameOver true, winner "". |
| E (invalid human move, no computer move) | X={0,1}, O={3,4} (2 vs 2; live O threat on cell 5) | occupied cell 0; also out-of-range 99; also malformed JSON | 400 each (identical bodies/statuses to today); state unchanged: still X={0,1}, O={3,4} (O count 2), X to move, not game over — no computer move, even though the game is in computer mode with an O threat available. |

Distribution sanity for the "uniformly random" claim is by construction — the default selector is a single `rand.Intn` over the list of empty cells — and the legality test over many real-randomness runs (fixture A) confirms every response places exactly one O in a legal empty cell without asserting any particular outcome.

Manual real-browser checklist (the delegated coordinator executes this independently as eval verification; it is the only UI verification, because no browser-automation dependency exists):

1. **Initial state / mode choice (AC-1, AC-7):** start `go run . -addr=localhost:8080`, open a new tab. The page shows a mode selector and "Two players", an empty board, and X to move. The selector offers exactly "Two players" and "Computer".
2. **Mode selection (AC-2):** click "Computer". The board becomes empty, the turn is X, the modal shows no result, and the mode display shows Computer.
3. **Paired move (AC-4, AC-8):** click an empty square. In the same user action the human X appears and then one O appears on a random empty square; afterwards it is X's turn and no square is clickable mid-reply more than once (double-click guard).
4. **Reset (AC-3):** click Reset Game. Board clears, X to move, no result, and the mode is still Computer.
5. **Refresh (AC-2, AC-7):** reload the page. The mode is still Computer and the game continues from the server state (board, turn, result reappear); refresh does not start a new game or switch modes.

Static-asset regression (existing tests, unchanged): the served HTML/CSS byte and marker checks (`TestHomePage_LinksStylesheetAndKeepsMarkup`, `TestStyleCSS_*`, `TestHomePage*`) keep passing and are understood strictly as checks that the served assets contain the documented elements and stay inline-style-free — they are not evidence of browser behavior.

Untested by design: actual browser rendering/interaction (covered by the coordinator's manual checklist), CLI behavior (unchanged, existing `main_test.go` covers it), and the computer-draw branch (unreachable by construction under the equal-counts invariant, as shown above; draw semantics are covered by existing tests and fixture D).

## Observability

The existing `loggingMiddleware` already logs method, path, and status per request, which is enough to see `/mode` switches and `/move` activity in the server log. Mode transitions are additionally visible in every subsequent GET /state payload. No new logging, metrics, dashboards, or alerting are introduced — the deployment envelope is a local single-process game with no monitoring infrastructure, and shipping new instrumentation would be disproportionate to this feature. Operating check: start the server, open the page, and confirm a normal session produces 200s on `/state`, `/move`, `/mode`, and `/reset` and coherent `mode` values in responses.

## Security and privacy considerations

No credentials, identity, entitlements, or access-control logic are touched. The feature adds no external calls, no new network exposure, no persistence, and no PII beyond what the page already handles (nothing). The mode value is validated against a two-value whitelist (anything else → 400, state unchanged). Randomness is used only for the computer's move, not for any security purpose, so `math/rand` is appropriate. The server continues to listen on the address main.go passes (README default is localhost), so the exposure posture is unchanged. No review beyond the risk tier is needed.

## Performance considerations

Load is a single-user local game. The computer move is an O(9) scan plus one `rand.Intn` inside the existing mutex hold; time under lock remains on the order of microseconds. Response sizes are unchanged apart from a short `mode` string. Under rapid clicks, requests serialize on the existing mutex and each is answered independently; the page's in-flight guard prevents duplicate submits. No memory, latency, or failure-mode budget concerns.

## Risk tier recheck

Ran the risk-tiering skill against what this plan actually describes, not what SPEC.md assumed. The change alters server-side game behavior and API state (automated computer moves, mode switching that restarts the game, reset semantics), so a mistake could silently change behavior without violating any entitlement — the definition of Tier 1. It touches no identity, entitlement, credential, or access-control logic (there is none in this module) and is not Tier 2. The browser-UI portion alone would be Tier 0, but it is subordinate to the server behavior, which dominates. The tier does not move down.

Risk tier after planning: 1
Changed from Planning and Requirements: no

## Rollback and migration

State is in-memory only; there is no migration and no persisted data. Rollback is a revert of the five touched files (server.go, index.html, style.css, server_test.go and/or its sibling test file): reverted, the server returns to exactly today's behavior — no mode, no computer moves, `/reset` clearing the board with X to move. Because the diff is small, self-contained, and additive (one new endpoint, one additive response field, one internal branch), a revert is trivially complete and requires no data repair. The test seam is unexported and dies with the revert; no test or harness that depends on external tooling survives it, since none is added.

## Dependencies and sequencing

No new modules or dependencies; stdlib only. `golangci-lint` v2 is already provisioned in the workload image (the team lint script refuses to run without it). No other work is in flight in this workspace; story order is internal to this task. The build loop sequence is: mode lifecycle first because the computer-move stories need `mode` and the `/mode` endpoint to exist; the riskiest story (US-002, the computer-move state machine and its deterministic-branch verification) immediately follows and never depends on a later story.

## Quality gates

Every story passes all four gates before it counts as done, run from /workspace (the Go module root). The builder runs them; the reviewer and the Watts checks run them again independently. `build.sh` writes its products to /runtime/build as private execution artifacts, not application source. The lint gate uses the team's `golangci.yml` via the team script; lint findings are fixed by changing code, never by weakening the lint configuration, and no replacement lint tool is installed.

- `bash /inputs/resources/team/scripts/build.sh`
- `go test ./...`
- `go vet ./...`
- `bash /inputs/resources/team/scripts/lint.sh`

## User stories

*The build loop works through these in order. Every SPEC criterion is covered: AC-1, AC-2, AC-3, AC-6, AC-7 in US-001; AC-4, AC-8 in US-002; AC-5 in US-003; AC-1/AC-7 UI halves in US-004; AC-6 page-presentation regression in US-005. US-001 is the minimal dependency for everything after it; the approach-proving risk (random computer move with terminal branches) lives in US-002, the next story.*

### US-001: Server mode state, lifecycle, and reset semantics

Covers: AC-1, AC-2, AC-3, AC-6, AC-7
Depends on: none

**As a** user opening the browser game
**I want** the server to know and report the game mode and to switch or reset modes predictably
**So that** the browser UI and tests can rely on a single server-side mode lifecycle

#### Acceptance Criteria
- [ ] `newGameState` starts in `"two-player"` with an empty board, X to move, and no result (AC-1); GET /state returns `mode: "two-player"`, and every state response includes the additive `Mode` field (AC-7). Existing state fields' output is unchanged.
- [ ] `POST /mode` with a different valid mode starts a fresh game in that mode: empty board, X to move, no result, and that mode in the response — verified mid-game and after a finished game (AC-2).
- [ ] `POST /mode` with the mode already active returns 200 with the current state and leaves the game untouched (spec assumption).
- [ ] `POST /mode` with any other body (missing field, unknown value, malformed JSON) returns 400 with `{"error": ...}` and leaves mode, board, turn, and result unchanged.
- [ ] `POST /reset` after moves and after a finished game clears the board and result, sets X to move, and keeps the active mode unchanged in both two-player and computer modes (AC-3).
- [ ] Two-player mode is byte-identical to today: the existing `/move` alternation tests (`TestMove_ValidCell`, `TestMove_AfterWin`, `TestMove_AfterDraw`, `TestReset`) all pass unchanged, no computer O ever appears in two-player mode, and API error behavior is unchanged (AC-6).
- [ ] All four quality gates pass.

### US-002: Computer move after each valid nonterminal human move

Covers: AC-4, AC-8
Depends on: US-001

**As a** solo human player
**I want** the computer to answer each of my valid, non-winning moves with exactly one random O
**So that** I can practice alone without strategy, difficulty levels, or external services

#### Acceptance Criteria
- [ ] In computer mode, a valid human X move that does not end the game results, in the same `/move` response, in the human's X mark plus exactly one O mark on a previously empty cell, with `currentPlayer` X and the game not over (AC-4).
- [ ] The default computer move source is the Go standard library only: `math/rand.Intn` over the list of currently empty cells; no strategy, difficulty, model calls, or external services (AC-8).
- [ ] Fixture A passes: each of many runs over the real random source returns a legal result — human X placed, exactly one O in an empty cell from {2,4,5,6,7,8} for the fixture board, X to move, not game over — with no assertion of any particular random O (AC-8 legality without outcome assumptions).
- [ ] Fixture B passes through the real handlers: with the seam forcing pick 5, the response reports O's win (gameOver true, winner O); with the seam forcing pick 6, the response reports O={3,4,6}, X to move, game not over — the same production branch deterministically covering both the computer-win terminal path and the nonterminal path (AC-4 terminal behavior, AC-8).
- [ ] The equal-counts invariant holds in every fixture: X to move implies `|X| == |O|` on every reachable pre-move board; after a nonterminal pair the counts are equal again with X to move. No fixture with unequal counts appears.
- [ ] All four quality gates pass.

### US-003: No computer move after invalid, winning, or draw-filling human moves

Covers: AC-5
Depends on: US-002

**As a** solo human player
**I want** the computer to stay silent when my move is invalid or already ends the game
**So that** the turn, board, and result are never corrupted by an automatic reply

#### Acceptance Criteria
- [ ] Fixture C passes: human X's winning move (X={0,1}, O={3,4}, human plays 2) ends the game immediately — exactly 3 X and 2 O marks (O count remains 2), gameOver true, winner X, no computer O (AC-5).
- [ ] Fixture D passes: human X's board-filling move (X={0,2,5,6}, O={1,3,4,8}, human plays 7) ends the game as a draw — exactly 5 X and 4 O marks, gameOver true, winner "", no computer O (AC-5).
- [ ] Fixture E passes: in computer mode with a live O threat, an occupied cell, an out-of-range cell, and malformed JSON each return the same 400 status/body as today and leave the state bit-for-bit unchanged — still |X|=2, |O|=2, X to move, not game over — with no computer O (AC-5).
- [ ] No reachable board in the suite allows the computer to fill the board (shown impossible under the equal-counts invariant: the human always makes the final filling move); draw semantics remain covered by existing tests and fixture D, and the plan claims nothing else.
- [ ] All four quality gates pass.

### US-004: Browser mode selector and mode display

Covers: AC-1, AC-7
Depends on: US-003

**As a** person opening the browser game
**I want** a visible two-player/computer choice and a mode readout
**So that** I can pick how to play and see, after refresh, that my chosen mode is still active

#### Acceptance Criteria
- [ ] The page offers exactly a "Two players" and a "Computer" control rendered in the existing visual style, styled only through `style.css` with no inline `<style>` and no changes to the existing board/status/reset markup and IDs that the current script and tests depend on.
- [ ] The script renders `state.mode` on initial load, after moves, after reset, and after a mode change, and shows the currently active mode.
- [ ] Changing the selector submits `{"mode": ...}` to `POST /mode` and re-renders the returned fresh state; selecting the already-active mode leaves the game alone; selecting the other mode shows an empty board with X to move.
- [ ] A page refresh re-reads state from GET /state and restores the board, turn, result, and mode without resetting the game or switching modes (AC-7); this is verified through the production `/state` handler tests plus the manual checklist — served-HTML string tests remain static-asset regression only and are not claimed to establish browser behavior.
- [ ] A lightweight in-flight guard prevents duplicate move submits while a paired `/move` response is pending.
- [ ] All four quality gates pass.

### US-005: Regression sweep, checklist artifact, and final gate run

Covers: AC-6
Depends on: US-004

**As a** reviewer and delegated evaluator
**I want** the full feature assembled, the CLI and existing page presentation proven untouched, and an executable browser checklist
**So that** independent human verification of the UI can proceed with no automation infrastructure

#### Acceptance Criteria
- [ ] `/workspace/main.go` contains no changes in this task's diff (CLI game, terminal-result handling, `parseInput`/`checkWin`/`isDraw` untouched); `main_test.go` passes unchanged (AC-6 CLI and terminal-result behavior).
- [ ] The complete `go test ./...` suite — existing CLI/board/server tests plus the new mode and computer-move tests — passes, and existing two-player alternation, API error behavior, and page asset tests are unchanged and green (AC-6).
- [ ] The real-browser manual checklist (Testing strategy above: mode selection, paired move, reset, refresh, initial state) is written to the task directory as `manual-browser-checklist.md` for the delegated coordinator to execute as independent eval verification; the plan claims no automated browser verification.
- [ ] All four quality gates pass from /workspace one final time; lint findings are fixed in code without weakening the lint configuration.

## Open questions for the author agent

Resolved in this plan, in writing, so they do not resurface mid-build:

- Mode labels and values: `"two-player"` and `"computer"` in JSON and in the UI controls' labels. The coordinator's checklist and the tests rely on these exact strings.
- Mode-selector placement: a radio-style two-option control above the board, below the status line, styled by `style.css`. Spec left placement to the plan.
- Invalid mode body: 400 `{"error": "invalid mode"}`, state untouched (matches existing error conventions).
- Refresh behavior: mode and game state come back from GET /state; no localStorage is used (mode is server state per the spec).
- Computer-draw unreachability: per the equal-counts invariant the human always makes the final filling move, so the suite does not chase an unreachable computer-fill branch; the computer-win branch is covered deterministically via the seam.

## Review

An agent drafts this document. The human approves or rejects it at the plan-review workflow gate with `wctl task decide <task> plan-review`, optionally using `--reject --feedback 'changes needed'`.