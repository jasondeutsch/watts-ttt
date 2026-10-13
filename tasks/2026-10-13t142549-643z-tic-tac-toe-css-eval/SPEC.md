# SPEC: Tic-tac-toe CSS refinement

## Status
Draft
Version: v2
Owner: j
Last updated: 2026-10-13

## Intent
Improve the presentation of the existing Go tic-tac-toe web UI without expanding
its functionality. The request is deliberately narrow: CSS in its own file,
updated colors, and a more visually impressive 3D presentation of the grid.

## Background and context
The application is a small Go module (`tic-tac-toe`, Go 1.21) with one rule
engine shared by two front ends:

- `index.html` (project root) is the entire web UI in one file: an inline
  `<style>` block with the current flat, gray-and-blue styling, and a `<script>`
  block that drives play through the HTTP API. It already sets a mobile viewport
  meta tag.
- `server.go` embeds the HTML with `//go:embed index.html` so a single binary
  serves the page, and its mux handles exactly four routes: `/` (the page),
  `/state` (GET), `/move` (POST), and `/reset` (POST). Any other path returns
  404; there is no static-asset route today.
- `main.go` implements terminal play, the win/draw checks, and the `-addr` flag
  that switches the binary to web mode.
- The current look is flat: a `#f5f5f5` page, white square cells, blue X and red
  O marks, and no depth cues. The only explicit focus style is on the reset
  button; the only motion is hover/active background changes.
- `server_test.go` builds a test-only mux whose `/` route returns placeholder
  HTML, so no existing test exercises the embedded asset bytes. `main_test.go`
  covers terminal rules and output.

This task is an iteration on that working codebase, not a new application. Only
the presentation layer and the minimal serving glue around it change; the game's
existing API and interaction are preserved.

## Acceptance criteria
- AC-1: When the browser loads the page, the Go application shall serve the
  page's styling from a separate CSS file (for example `/style.css`, with a
  `text/css` content type); the served HTML shall contain no inline `<style>`
  block, and the binary built with the existing `embed` mechanism shall contain
  both the page and the stylesheet.
- AC-2: When the board is displayed, the palette and grid shall visibly improve
  on the baseline, with CSS depth cues such as layered shadows, beveled or raised
  cells, and dimensional borders. The nine playable cells shall remain legible.
- AC-3: When players make legal moves, reach a win/draw, or reset, the existing
  behavior and feedback shall remain intact. API schemas and terminal gameplay
  shall remain unchanged.
- AC-4: At mobile and desktop widths, the board and controls shall remain usable
  without page overflow. Keyboard focus shall remain visible; any new motion
  shall respect reduced-motion preferences.
- AC-5: The implementation shall stay limited to the stylesheet, the minimal
  HTML and Go asset embedding/serving changes required to use it, and focused
  asset-serving tests if needed. Existing regression tests shall still pass.

## Explicit non-goals
No new gameplay modes, animation system, theme switcher, frontend framework,
external asset/CDN dependency, account system, new game rules, or general code
cleanup. Do not redesign the Go game architecture or change JavaScript gameplay.
Do not edit prior task directories, workflow configuration, or evaluation records.
Do not publish build binaries into the project.

## Assumptions
The coding Agent may choose the exact palette and CSS depth treatment within the
request. Subtle raised tiles and a coherent modern palette are sufficient; 3D
means CSS presentation, not WebGL or a graphics engine. There are no unresolved
product decisions requiring a new feature or broader work.
The target browsers are modern evergreen browsers; the page already relies on
CSS Grid and `aspect-ratio`, so features such as `prefers-reduced-motion` are
available to the stylesheet.

## Constraints
Application changes belong in the project root; task documents and execution
evidence belong in this task folder. Use existing model/container/tool policy.
Prefer a small maintainable stylesheet.
Keep single-binary asset embedding with the existing `go:embed` approach; do not
add an external asset or CDN dependency (also a non-goal).
Keep the document's element IDs and classes intact — `#board`, `#status`,
`#reset-btn`, `.cell`, `.taken`, `.x`, `.o`, `.game-over`, and the status
classes — because the inline script and existing tests depend on them; the
JavaScript gameplay code shall not change.
Keep `/state`, `/move`, `/reset` routes, their methods, and JSON schemas
unchanged; keep terminal mode (the default `go run .`) unchanged. Serve the
stylesheet with the correct `text/css` content type.
Do not install or replace the Go toolchain. Do not modify or republish the
`ttt` binary already present in the project root, and do not add new build
artifacts.

## Open questions
- Non-blocking, delegated for this eval: the existing server tests stub the root
  route with placeholder HTML (`setupServer` in `server_test.go`), so the
  embedded assets are untested. Should the deliverable include a focused
  asset-serving test (for example, `GET /style.css` returns 200 with
  `text/css`, and the served HTML links the stylesheet), or is manual browser
  verification enough? AC-5's "if needed" leaves this open; PLAN.md should
  state the choice.
- Visual details (exact palette, depth treatment) are delegated to the
  implementer and are not blockers.

## Risk-relevant context
- Touches: `index.html` (stylesheet link, removal of inline styles), a new
  stylesheet file, the `go:embed` directive and static-file route in
  `server.go`, and (if chosen) focused asset-serving tests.
- Does not touch: game state/rules, move/reset API, terminal interface, auth, data persistence.

## Risk tier
0 (presentation and static asset integration; the workflow Agent should recheck).

## Review
SPEC refinement and planning are workflow-owned. Human approvals are delegated
to the external Codex coordinator for this eval, solely against the scope above.
