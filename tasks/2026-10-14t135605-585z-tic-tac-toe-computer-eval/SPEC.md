# SPEC: Optional random computer opponent for browser tic-tac-toe

## Status
Draft
Version: v3
Supersedes: none
Owner: j
Last updated: 2026-10-14

## Intent

A person opening the browser game in a new tab currently can only play tic-tac-toe against another human on the same page. The request adds an optional computer opponent: the user can choose to play against the computer, which replies to each valid nonterminal human move with exactly one random legal move. This gives a single human player a usable solo practice mode without any strategy, difficulty, or external services, while keeping the existing two-player browser mode, the command-line game, and the current look and behavior intact. The user can switch modes at any time; switching starts a fresh game in the newly selected mode.

## Background and context

The project is a standalone Go module (`tic-tac-toe`, Go 1.21) in /workspace with a single `main` package:

- `main.go` — CLI game (`playGame`, X first, alternating turns, win/draw/abandon handling) and shared board logic (`newBoard`, `render`, `parseInput`, `checkWin`, `isDraw`). Board cells are indices 0–8; CLI prompts use positions 1–9.
- `server.go` — production HTTP server via `newServerMux(gs)`: serves embedded `index.html` and `style.css`, `GET /state`, `POST /move`, `POST /reset`. One server-wide `gameState` (`board`, `currentPlayer`, `winner`, `gameOver`) guarded by a mutex. `/move` validates JSON body, presence and range of `cell` (0–8), occupancy, and `gameOver`; it then places the mark, checks win/draw, and toggles the player. Errors return HTTP 400 with `{"error": ...}` or the current state. The state response and the page script know nothing about a mode today.
- `index.html` — static page: 3×3 board buttons, `#status` line, Reset Game button; no mode selector today.
- `style.css` — all styling is in the separate stylesheet (no inline `<style>`).
- `main_test.go` / `server_test.go` — existing tests cover board logic, CLI play, and the HTTP handlers through `newServerMux`; tests already invoke the exact production handlers.

Today there is no notion of mode: every valid move alternates X then O, X always starts, and `/reset` clears the board and sets the turn to X. The request adds a mode concept to this existing server-wide single-game state; it does not ask for any new architecture.

## Acceptance criteria

- AC-1: When a user opens the browser game with no prior game state, the server shall start the game in two-player mode with an empty board and X to move; the page shall offer the user a choice of two-player mode or computer mode.
- AC-2: When the user selects a mode different from the currently active mode at any time, during a game or after one ends, the server shall start a fresh empty game in the selected mode with X to move and no result.
- AC-3: When the user resets the game, the server shall clear the board and result, set the turn to X, and keep the currently selected mode unchanged.
- AC-4: While the game is in computer mode, when the human submits a valid move that does not end the game, the server shall place the human's X mark and then place exactly one computer O mark on a uniformly random empty cell, returning the state after both marks with X to move; if the computer's move wins the game or fills the board, the server shall mark the game over and set the winner or draw result.
- AC-5: While the game is in computer mode, if the human submits an invalid move (malformed request, out-of-range cell, or occupied cell), then the server shall make no computer move and shall leave the board, turn, and result unchanged; when the human's valid move itself wins the game or fills the board with no winner, the server shall end the game without a computer move.
- AC-6: While the game is in two-player mode, the server shall continue to alternate X and O on each valid move, with X first and no computer move; the command-line game, terminal-result handling, API error behavior, and the existing page presentation shall work as they do today.
- AC-7: The server's state responses shall identify the active mode so the page can display the current mode, a page refresh keeps the selected mode, and tests can verify mode behavior through the production handlers.
- AC-8: The computer move shall use simple random number generation from the Go standard library, with no strategy, difficulty levels, model calls, or external services; the test suite shall exercise the production handlers or production game logic and shall verify random-move legality and terminal behavior without assuming any particular random move.

## Explicit non-goals

- No minimax, heuristic strategy, difficulty levels, model calls, or external services for the computer's move.
- No session, per-browser, or multiplayer architecture; the existing server-wide single-game state remains the only state.
- No changes to the command-line game; computer mode exists only in the browser UI.
- No change to two-player turn order, win/draw/reset semantics, or existing API error behavior.
- No persistence of mode or game state across server restarts.
- No undo, move history, or spectator/viewer features.

## Assumptions

- The computer's random move is generated by the production Go server code in the same request that applies the human's move, so the random-move legality and terminal behavior are testable through the real HTTP handlers (the required tests cannot exercise client-side JavaScript).
- "Random available square" means uniformly random among the currently empty cells, using ordinary random number generation supplied by the Go standard library.
- In computer mode the human is always X and moves first; the computer is always O and never takes the first move of a game.
- The mode is part of the server's single game state, so after the user picks a mode it stays selected for subsequent moves and resets until the user changes it.
- Selecting the mode that is already active has no effect on the current game; only selecting a different mode starts a fresh game (AC-2).

## Constraints

- Keep changes minimal in the project root; no new packages or modules.
- Keep the UI simple and use the existing visual style and the separate `style.css`; do not inline styles. The placement and visual design of the mode selector on the page are left to the plan.
- After each valid nonterminal human move in computer mode, exactly one automatic computer move occurs; no computer move may follow an invalid, winning, or draw-ending human move.
- The implementation must remain small; tests must exercise actual production handlers or game logic and must not assume a particular random outcome.
- The existing server-wide single-game state is sufficient; do not add session or persistence machinery.

## Open questions

None remain. The mode lifecycle is settled: initial mode is two-player (AC-1), selecting a different mode at any time starts a fresh game in that mode with X first (AC-2), and Reset preserves the active mode (AC-3). Page-layout placement of the mode selector is a plan decision, not a spec decision.

## Risk-relevant context

- Touches: /workspace/server.go (mode field on the game state, mode selection and switch handling, `/move` and `/reset` behavior, state responses), /workspace/index.html (mode selector and script), /workspace/style.css (styling for the new control), new tests alongside /workspace/server_test.go.
- Does not touch: /workspace/main.go CLI path and board logic, go.mod, external services, network exposure, credentials, or any entitlement/identity logic.
- Tier rationale: the change alters server-side game behavior and API state (automated computer moves, mode switching that restarts the game, reset semantics), so a mistake could silently change behavior; it touches no identity, entitlement, credential, or access-control logic, so it is Tier 1 rather than Tier 0 or Tier 2.

## Risk tier
1

## Review

An agent drafts this document. The human (j) approves or rejects it at the spec-review workflow gate with `wctl task decide <task> spec-review`, optionally using `--reject --feedback 'changes needed'`. The planning agent owns PLAN.md; user stories there must map to the AC-1..AC-8 IDs above.