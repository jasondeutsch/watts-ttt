# PLAN: Tic-tac-toe CSS refinement

Task documents: /runtime/task (SPEC.md, PLAN.md, story logs, review artifacts, execution evidence)
Implementation root: /workspace (the application workspace supplied by Watts; contains go.mod)

Application source, tests, and build files belong in /workspace and its normal source directories. The task directory stores workflow documents and evidence only; no application code or separate module is created there. Quality gates run from the project root (/workspace), never from the task directory.
Spec ref: SPEC.md v2
Risk tier at Planning: 0 (from SPEC.md)

*The planning agent drafts this file. The plan-review workflow gate records human approval before implementation starts.*

## Architecture and approach

Mechanical move plus minimal Go glue, no architecture change:

1. **Extract the stylesheet.** Move the entire inline `<style>` block from `index.html` byte-for-byte into a new file `/workspace/style.css`. Replace the block in `index.html` with a single `<link rel="stylesheet" href="style.css">` in `<head>`. No markup, element IDs, classes, or `<script>` content change. (US-001 preserves current styling; US-002 then restyles the contents of `style.css` only.)
2. **Embed the stylesheet.** Change the directive in `server.go` from `//go:embed index.html` to embed both files into the existing `embeddedFiles embed.FS`, keeping the single-binary `go:embed` approach.
3. **Shared asset-serving handler.** Add one small handler, `serveEmbeddedFile(name, contentType string) http.HandlerFunc`, that reads `name` from `embeddedFiles`, returns 500 with a logged error when the read fails, sets the given `Content-Type`, and writes the bytes with 200. Both the `/` route (guarded by the existing exact-path 404 check, serving `index.html` with `text/html; charset=utf-8`) and a new `/style.css` route (serving `style.css` with `text/css`) call this same handler. Method discipline matches the existing routes (GET only; others get 405). No `http.FileServer`, no new dependency, no external asset.
4. **Production mux factory.** Extract the mux construction from `startWebServer` into `newServerMux(gs *gameState) *http.ServeMux`, containing exactly the current four routes (`/`, `/state`, `/move`, `/reset`) with `/` and `/style.css` served via `serveEmbeddedFile`. `startWebServer` becomes `loggingMiddleware(newServerMux(newGameState()))` + `ListenAndServe`; route logic, JSON schemas, and validation are untouched. This is the only server restructuring, and it exists solely so tests can exercise the production handlers.
5. **Focused asset-serving tests (required, per plan review).** Repoint `setupServer` in `server_test.go` to `httptest.NewServer(newServerMux(newGameState()))`, deleting the placeholder-HTML duplicate mux, so every existing server test runs against the real production handlers. Add focused tests: `GET /style.css` returns 200, `Content-Type: text/css`, and bytes exactly equal to `embeddedFiles.ReadFile("style.css")`; `GET /` returns 200 `text/html` whose body contains the stylesheet link and no inline `<style`, and preserves the documented IDs/classes and the unchanged gameplay script. No test copies route logic and no test builds a different `http.FileServer`.

Go 1.21 is declared in go.mod, so the newer method-pattern mux syntax (`GET /style.css`) is not available; the route checks the method explicitly, consistent with the existing handlers.

## Data model and interface changes

None to the public surface: `/state`, `/move`, `/reset` methods, JSON schemas, and terminal mode are unchanged. The only new public path is `/style.css` (static, `text/css`). Internal additions in `server.go`: `serveEmbeddedFile(name, contentType string) http.HandlerFunc` and `newServerMux(gs *gameState) *http.ServeMux`, plus the embed directive change. New file: `style.css`. Changed files: `index.html`, `server.go`, `server_test.go`.

## Systems touched

Cross-checked against the risk-tiering skill, not from memory.

- Touches: `index.html` (stylesheet link in place of inline `<style>`), new `style.css` at project root, `server.go` (`//go:embed` set, shared asset handler, `/style.css` route, mux factory extraction), `server_test.go` (setup repointed at production mux, new focused asset-serving tests).
- Does not touch: game state/rules (`main.go` rules, `checkWin`, `isDraw`), the move/reset/state API, the JavaScript gameplay code, terminal interface, `main_test.go`, `go.mod`, `README.md`, the checked-in `ttt` binary, auth, persistence, governance or identity data.

## Technical decisions and alternatives considered

- **Shared `serveEmbeddedFile` over `http.FileServer` or a standalone CSS rule about which route serves what.** `http.FileServer` was rejected: it changes 404 semantics, would need a root-embedding scheme to reach the single binary, and is a different serving path than `/` uses; the review explicitly forbids tests exercising a `http.FileServer` the production mux does not use. A single shared handler keeps one code path for both routes and is what the focused tests exercise.
- **Production mux factory (`newServerMux`) over spawning the built binary or keeping the test-only mux.** Spawning the executable was rejected as heavier and flaky (port management) and it can only be built as a private artifact via `build.sh`; the review explicitly allows "actual production asset-serving handler". Keeping the copied test-only mux was rejected outright by review feedback (tests must not exercise a duplicated handler). The factory is the minimal restructuring that lets tests and production share the identical handlers; gameplay and API code are not refactored.
- **Move CSS verbatim first (US-001), restyle later (US-002) over one combined story.** Keeps US-001 behavior-preserving and reviewable (CSS values identical to baseline) and makes the visual change a single, separately checkable story. Per review, US-001 is gated on preserved selectors and gameplay integration, not on identical final CSS properties.
- **`style.css` at project root next to `index.html`** matches SPEC's example path (`/style.css`), keeps the single-file embed pattern simple, and requires no directory restructure.
- **No inline style fallback.** The page is served only by this binary, so the external stylesheet is always present; no `<noscript>`-style CSS duplication is warranted.

## Testing strategy

- **Focused asset-serving tests (US-001, required).** `GET /style.css` → 200 + `text/css` + byte-for-byte equality with the embedded bytes; `GET /` → 200 + `text/html` + contains the stylesheet link + contains no inline `<style` + preserves the documented IDs/classes and the unchanged script. These run against `newServerMux` via `httptest`, the exact handlers production serves.
- **Existing regression suite (US-003).** All current `server_test.go` and `main_test.go` tests stay, with `setupServer` repointed at `newServerMux(newGameState())`. They cover move validation, win/draw, post-game rejection, reset, state hydration, 404 behavior, and terminal rules/output, and now run against the real production handlers.
- **Explicitly not tested in CI:** real-browser rendering of the palette/depth cues. This is delegated to the implementer's judgment per SPEC assumptions; Go-level gates cannot assert pixels, and this is a low-risk, immediately visible, trivially revertable change (Tier 0). Selector and markup preservation is asserted in tests instead.
- Tests use only `net/http/httptest` and the standard testing harness; no new dependencies, no wall-clock sleeps, no live services.

## Observability

The existing `loggingMiddleware` already logs `METHOD path status` for every request, so `GET /style.css` hits, asset 404s, and the rare embedded-read 500 (also logged explicitly in `serveEmbeddedFile`) are visible in stdout logs. No new metrics or instrumentation; nothing else changes.

## Security and privacy considerations

No user data, auth, or entitlements involved. Static first-party assets served under explicit `Content-Type` headers (`text/html; charset=utf-8`, `text/css`) to avoid MIME sniffing. No external/CDN dependency is introduced (also a non-goal). Nothing here needs review beyond the risk tier.

## Performance considerations

Negligible: static bytes served from memory via `embed.FS`; no disk I/O, no new dependencies, no added network calls, and a single extra in-binary file. Failure mode is already handled: a missing embedded file returns a logged 500 like today. No latency or memory budget concerns; there is no backend load to speak of.

## Risk tier recheck

Run against what this plan actually does: move a stylesheet, add one static route, and repoint tests at the production mux. Cosmetic, immediately visible, trivially reverted; zero touch on game state, entitlements, routing behavior beyond adding a static path, or credentials.

Risk tier after planning: 0
Changed from Planning and Requirements: no
If yes, why: n/a

## Rollback and migration

Not required at Tier 0, but trivial: revert `index.html`'s link to the inline `<style>`, delete `style.css` and the `/style.css` route, restore the embed directive, and restore `setupServer`; no data or schema migration exists. Nothing ships outside the built binary; the page and stylesheet are always embedded together in the same commit.

## Dependencies and sequencing

No external dependencies, no network use during build/test/lint. Embedded assets make runtime file presence a non-issue. No other work is in flight in this workspace; sequencing is purely story-internal (US-001 then US-002 then US-003 per the dependency lines below).

## Quality gates

Every story must pass all four gates before it counts as done; the reviewer and Watts checks run them again independently from the project root. These are the build Agent's declared tools; no separate lint stage, no replacement tools, and lint findings are fixed without weakening the lint configuration.

- `bash /inputs/resources/team/scripts/build.sh`
- `go test ./...`
- `go vet ./...`
- `bash /inputs/resources/team/scripts/lint.sh`

## User stories

*The build loop works through these in order. Keep the heading format exactly (`### US-001: Title`). Each story is small enough for one agent iteration, has checkable acceptance criteria, a `Covers:` line naming SPEC criteria, and a `Depends on:` line.*

### US-001: Serve the page's styling from a separate embedded CSS file

Covers: AC-1, AC-5
Depends on: none

**As a** web player
**I want** the page's styling to be served from its own CSS file with the correct content type, embedded in the same single binary
**So that** the page still works from one binary with no inline style block and no external dependency.

#### Acceptance Criteria
- [ ] `index.html` contains no `<style>` block and links exactly one stylesheet, `style.css` (`<link rel="stylesheet" href="style.css">` in `<head>`); structural markup and the `<script>` gameplay block are unchanged.
- [ ] `style.css` exists at the project root and contains the styling (initially the current rules moved verbatim from the inline block; final property values are intentionally restyled in US-002).
- [ ] `server.go` embeds both `index.html` and `style.css` via `//go:embed` into `embeddedFiles`, and the binary built by the `build.sh` gate contains both files.
- [ ] A shared `serveEmbeddedFile(name, contentType)` handler is added in `server.go` and used by the `/` and `/style.css` production routes; `GET /style.css` returns 200, `Content-Type: text/css`, and the exact embedded bytes.
- [ ] `newServerMux(gs)` is extracted in `server.go` and used by `startWebServer`; the four production routes remain exactly as today except `/` and `/style.css` share the asset handler.
- [ ] Focused asset-serving tests exercise the production handlers via `httptest.NewServer(newServerMux(newGameState()))` (no copied/test-only mux, no `http.FileServer`): `GET /style.css` → 200 + `text/css` + byte-for-byte embed match; `GET /` → 200 + `text/html` + body contains the stylesheet link + contains no inline `<style` (AC-1 asserted in Go).
- [ ] The served `/` body preserves the documented IDs/classes (`#board`, `#status`, `#reset-btn`, `.cell`, `.taken`, `.x`, `.o`, `.game-over`) and the unchanged gameplay script, so selectors and gameplay integration are intact — this is the US-001 bar, not identical final CSS properties (US-002 changes them deliberately).
- [ ] All four quality gates pass from /workspace.

### US-002: Restyle the board with depth cues and a coherent palette

Covers: AC-2, AC-4
Depends on: US-001

**As a** web player
**I want** the board to look modern and dimensional with a coherent palette
**So that** the game feels polished while remaining legible, keyboard-accessible, and overflow-free.

#### Acceptance Criteria
- [ ] Only `style.css` changes for presentation; `index.html` markup, IDs/classes, and the script remain exactly as US-001 left them (gameplay integration preserved).
- [ ] The board shows clear CSS depth cues — layered shadows, beveled or raised cells, dimensional borders — and the palette is coherent and visibly improved over the baseline flat gray-and-blue.
- [ ] The nine playable cells remain legible: X and O marks stay distinguishable with sufficient contrast on their cells.
- [ ] The board and controls remain usable and do not cause page overflow at mobile widths (existing `aspect-ratio` grid + a responsive media query) and at desktop widths.
- [ ] Keyboard focus stays visible on cells and the reset button (e.g., `:focus`/`:focus-visible` outline).
- [ ] Any new transitions/animations respect `prefers-reduced-motion`.
- [ ] All four quality gates pass from /workspace.

### US-003: Verify gameplay, API schema, and terminal behavior are unchanged

Covers: AC-3, AC-5
Depends on: US-001

**As a** maintainer
**I want** evidence that only the presentation layer changed
**So that** the existing API contract, terminal gameplay, and regression coverage are provably intact.

#### Acceptance Criteria
- [ ] `setupServer` is repointed at `newServerMux(newGameState())` and the placeholder-HTML duplicate mux is deleted, so every server test exercises the actual production handlers.
- [ ] `/state`, `/move`, `/reset` remain at the same paths, methods, and JSON schemas with identical response bodies (`server_test.go` passes unchanged aside from the setup replacement); move validation, win/draw detection, post-game rejection, reset, and state hydration behave exactly as before.
- [ ] Terminal gameplay (`main.go` and `main_test.go`) is untouched and `main_test.go` passes.
- [ ] `/style.css` is the only new public path; unknown paths still return 404; no API field or endpoint was added or removed.
- [ ] Scope check: the diff touches only `index.html`, `style.css`, `server.go`, `server_test.go` (including the new asset tests); `main.go`, game rules, JS gameplay, `go.mod`, `README.md`, and the checked-in `ttt` binary are untouched; no build artifacts are committed (the `build.sh` gate writes only to its private runtime directory).
- [ ] All four quality gates pass from /workspace.

## Open questions for the author agent

None. The previously open test question is resolved by plan review: focused asset-serving tests are required (US-001) and they must exercise the production asset-serving handlers via `newServerMux`, exactly as described above. The palette and depth treatment are delegated to the implementer by SPEC's Assumptions and are implementation choices, not open product questions. The shared asset-handler and mux-factory approach is fixed in Technical decisions and alternatives considered; no unresolved implementation choices remain for the build Agent to invent.

## Review

An agent drafts this document. The human (delegated to the external Codex coordinator for this eval) approves or rejects it at the plan-review workflow gate. This revision incorporates feedback from the prior plan-review rejection: scope stays narrow, asset-serving tests are required and run against the production handlers, US-001 is gated on preserved selectors and gameplay integration rather than identical final CSS values, and the previously flagged question is resolved rather than left open.