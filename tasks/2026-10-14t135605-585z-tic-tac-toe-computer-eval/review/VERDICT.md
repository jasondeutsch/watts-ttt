# Review Verdict

Task: Optional random computer opponent for browser tic-tac-toe
Reviewed: /workspace against /runtime/task/SPEC.md (v3) and /runtime/task/PLAN.md
Date: 2026-10-14
Reviewer: delegation review loop (iteration 1)

## Summary

The implementation satisfies all eight SPEC acceptance criteria through the
production handlers, and all four declared quality gates pass on the final
candidate. The diff is confined to the five planned touch points
(server.go, index.html, style.css, server_test.go, plus the sibling test files
computer_test.go and mode_test.go); main.go, main_test.go, go.mod and the CLI
are untouched. Verdict: **PASS**, with two minor non-blocking findings below.

## Evidence: quality gates (run from /workspace)

| Gate | Command | Result |
|---|---|---|
| Build | `bash /inputs/resources/team/scripts/build.sh` | PASS — exit 0, product in `/runtime/build/tic-tac-toe` |
| Test | `go test ./...` and `go test ./... -count=1` | PASS — `ok tic-tac-toe`, no cache |
| Vet | `go vet ./...` | PASS — clean |
| Lint | `bash /inputs/resources/team/scripts/lint.sh` (golangci-lint 2.14.0, team config) | PASS — 0 issues |

Additional verification: live smoke of the production binary
(/runtime/build/tic-tac-toe -addr=127.0.0.1:18099) exercised GET /state,
POST /mode, POST /move, POST /reset, GET /mode (405), and served /
through real HTTP. 13/13 checks passed: initial two-player empty-board state,
fresh game on mode switch, invalid mode 400 with state unchanged, paired
X+O move with X to move, occupied-cell 400 with no computer reply and state
unchanged, reset preserving computer mode, `mode` field present in state
responses, and the page offering exactly the two mode controls with no inline
`<style>`. No app source was modified during review; build products and the
smoke script were kept under /runtime.

## Acceptance-criteria checklist

- AC-1 (initial two-player mode, empty board, X; page offers both modes): PASS —
  `newGameState` defaults to `modeTwoPlayer`; `TestState_InitialModeIsTwoPlayer`,
  `TestHomePage_IncludesModeSelectorAndGuard`; smoke (initial state + page HTML).
- AC-2 (different mode at any time starts a fresh game, X to move, no result):
  PASS — `POST /mode` resets board/winner/gameOver and sets the mode;
  `TestMode_SwitchStartsFreshGameMidGame`, `TestMode_SwitchStartsFreshGameAfterGameOver`,
  `TestMode_AlreadyActiveHasNoEffect` (no-op for active mode), `TestMode_InvalidBodies`
  (400 `{"error":"invalid mode"}`, bit-for-bit state unchanged), `TestMode_NonPostMethod` (405).
- AC-3 (reset clears board/result, X to move, keeps mode): PASS — `POST /reset`
  preserves `gs.mode`; `TestReset_PreservesModeTwoPlayer`,
  `TestReset_PreservesModeComputer`, `TestReset_PreservesModeComputerAfterGameOver`.
- AC-4 (computer mode: human X + exactly one random O per valid nonterminal move,
  X to move after; terminal computer move sets result): PASS — `/move` runs the
  computer step in the same request/API mutex hold; `TestComputerMove_FixtureA_RealRandomLegality`
  (50 runs over real randomness asserting legality only), `TestComputerMove_FixtureB_SeamTerminalBranches`
  (seam pick 5 → computer win, winner "O"; pick 6 → nonterminal pair).
- AC-5 (no computer move after invalid/winning/draw-filling human move): PASS —
  all validations return before the computer branch on the same go-routine;
  fixture C (human win, exactly 3 X / 2 O), fixture D (human fills board, draw,
  exactly 5 X / 4 O), fixture E (occupied/out-of-range/malformed 400s with the
  pre-existing messages and bit-for-bit unchanged state despite a live O threat).
- AC-6 (two-player behavior byte-identical; CLI/API/page unchanged): PASS —
  `TestTwoPlayer_NeverComputes`, full pre-existing `server_test.go`/`main_test.go`
  suite passes unchanged; computer branch is gated on `gs.mode`, two-player path
  logic (win/draw/toggle) is untouched; main.go contains no mode/computer references.
- AC-7 (state responses identify active mode): PASS — additive `Mode` field
  (`json:"mode"`) in every state response; `TestState_ModeFieldPresentThroughout`,
  `TestGetState_RefreshHydration`; smoke.
- AC-8 (stdlib random only, no strategy/services; tests exercise production
  handlers and assert legality/terminal behavior without assuming a random
  outcome): PASS — `defaultPickComputerMove` uses `math/rand.Intn` over empty
  cells (stdlib only; `go.mod` unchanged); the unexported `pickComputerMove` seam
  is used only to force legal picks through the real handlers; fixture A asserts
  legality across many real-random runs without asserting any specific cell.
- Non-goals respected: no minimax/difficulty/external services; no sessions or
  persistence (state in memory only); no CLI changes; no new packages/modules;
  styling stays in the separate style.css, no inline `<style>`.

## PLAN user-story coverage

US-001 (AC-1, AC-2, AC-3, AC-6, AC-7): covered and gated by mode_test.go +
existing suite. US-002 (AC-4, AC-8): covered by computer_test.go fixtures A/B.
US-003 (AC-5): covered by fixtures C/D/E. US-004 (AC-1/AC-7 UI halves): page
offers exactly the two radio controls and a mode display, wired to `POST /mode`
via `state.mode` with the `moveInFlight` guard; served-HTML assertions are
static-asset checks only, as planned. US-005 (AC-6 regression sweep): main.go /
main_test.go untouched; full suite green; all four gates green.

## Findings (minor, non-blocking)

1. Stray build artifacts in the application root: `/workspace/tic-tac-toe` and
   `/workspace/ttt` binaries. `/workspace/tic-tac-toe` is byte-identical
   (sha256 `30d82be7...`) to the `/runtime/build/tic-tac-toe` gate product, so it
   was built from the same source/toolchain during the build step; `/workspace/ttt`
   matches the README-documented `go build -o ttt .` product (HYPOTHESIS: pre-task
   artifact). This is against the "build products and caches under /runtime"
   convention and would be untracked junk in a git repo, but it has no effect on
   any gate or on runtime behavior. Reviewer left the files in place per
   "do not change the application"; the executor may remove them before shipping.
2. PLAN.md US-005 says the real-browser manual checklist is written to the task
   directory as `manual-browser-checklist.md`; that standalone file does not
   exist. STORY_LOG.md states the plan-review gate approved omitting it (checklist
   already lives in PLAN.md, and the pinned build manifest publishes STORY_LOG.md
   only). No record of that plan-review decision is present in /runtime, so the
   approval claim is unverifiable (HYPOTHESIS). Non-blocking: the same checklist
   is present inline in PLAN.md (Testing strategy) and in STORY_LOG.md, so the
   delegated coordinator can execute it; SPEC.md itself does not require the file.

## Constraints/notes

- `go test -race` is unavailable in this image (CGO_ENABLED=0, no C compiler),
  as STORY_LOG.md documents; race detection is not one of the four declared
  gates. All new fixture tests drive sequential requests through the production
  handlers and never mutate the seam concurrently.
- OPEN_QUESTIONS.md contains no remaining P0/P1 items.
- No application files were modified by this review; no verification tools were
  installed or substituted for the declared gates.

VERDICT: PASS