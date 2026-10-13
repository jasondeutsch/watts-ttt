# Review Verdict: Tic-tac-toe CSS refinement

**Reviewer:** Ralph (iteration 1)
**Review of:** Implementation at /workspace against SPEC.md v2 and PLAN.md
**Risk tier:** 0 (confirmed — presentation and static asset integration only)
**Date:** 2026-10-13

## Gate results

| Gate | Status | Evidence |
|------|--------|----------|
| `bash /inputs/resources/team/scripts/build.sh` | **PASS** | `go build -o /runtime/build/ ./...` exited 0; binary produced to `/runtime/build` |
| `go test ./...` | **PASS** | `ok tic-tac-toe 0.092s` — all tests pass, including new focused asset-serving tests |
| `go vet ./...` | **PASS** | No output (exit 0) |
| `bash /inputs/resources/team/scripts/lint.sh` | **PASS** | golangci-lint v2.14.0 reported 0 issues |
| `node check.mjs spec` | **PASS** | `spec: OK` |
| `node check.mjs plan` | **PASS** | `plan: OK` |
| `node check.mjs stories` | **PASS** | `stories: OK` — all three stories (US-001, US-002, US-003) marked DONE in STORY_LOG.md |

## SPEC acceptance criteria audit

| Criterion | Verdict | Findings |
|-----------|---------|----------|
| **AC-1** (separate CSS file, embedded, no inline `<style>`) | **PASS** | `index.html` links `style.css` and has no `<style>` block; `server.go` uses `//go:embed index.html style.css`; `/style.css` route serves with `text/css`; `TestHomePage_LinksStylesheetAndKeepsMarkup` and `TestStyleCSS_ServedWithTextCSSContentType` confirm. |
| **AC-2** (depth cues, improved palette, legible cells) | **PASS** | `style.css` uses layered `--raised-shadow` / `--sunk-shadow`, gradient-beveled cells, dimensional borders, coherent slate-on-ivory palette; X (#1d4ed8) and O (#e11d48) marks have strong contrast and text-shadow glows. |
| **AC-3** (gameplay/API/terminal unchanged) | **PASS** | All existing regression tests pass (`TestMove_ValidCell`, `TestMove_AfterWin`, `TestReset`, `TestPlayGameXWinsDiagonal`, etc.); `main.go` and `main_test.go` untouched; `setupServer` exercises real production handlers via `newServerMux`. |
| **AC-4** (responsive, focus, reduced-motion) | **PASS** | Media query at 420px, `aspect-ratio` grid, `:focus-visible` on cells, `:focus` on reset button, `prefers-reduced-motion: reduce` disables transitions/animations. No page overflow at any width. |
| **AC-5** (scope limited to CSS/serving changes, tests pass) | **PASS** | Diff touches only `index.html`, `style.css` (new), `server.go`, `server_test.go`; `main.go`, `main_test.go`, `go.mod`, `README.md`, `ttt` binary are unchanged. All four quality gates pass. |

## PLAN user stories audit

| Story | Verdict | Evidence |
|-------|---------|----------|
| **US-001** (separate embedded CSS file) | **PASS** | `index.html` links external stylesheet, no inline `<style>`; `style.css` exists at project root; `server.go` embeds both files, has `serveEmbeddedFile` and `newServerMux` factory; tests confirm bytes, content type, markup preservation, and 405 rejection. Build/test/vet/lint all pass. |
| **US-002** (restyle with depth cues) | **PASS** | Only `style.css` changed; depth cues (layered shadows, beveled raised cells, gradient panel) implemented; cells legible with contrasting X/O; responsive, keyboard-focus visible, reduced-motion respected. All gates pass. |
| **US-003** (verify unchanged behavior) | **PASS** | `setupServer` uses `newServerMux(newGameState())`; all existing server and terminal regression tests pass; `/style.css` is only new path; unknowns 404; scope limited to 4 files; `main.go`, `main_test.go`, `go.mod`, `ttt` binary untouched. All gates pass. |

## Quality gate details

### `bash /inputs/resources/team/scripts/build.sh`
- Build written to `/runtime/build/`, not into `/workspace`
- Exit code 0
- Binary not published into the project root

### `go test ./...` (selected highlights)
- `TestStyleCSS_ServedWithTextCSSContentType`: GET /style.css → 200, Content-Type: text/css, bytes equal to embedded file. **PASS**
- `TestStyleCSS_RejectsNonGet`: POST /style.css → 405. **PASS**
- `TestHomePage_LinksStylesheetAndKeepsMarkup`: Served HTML contains `<link rel="stylesheet" href="style.css">`, no inline `<style`, preserves all documented IDs/classes and gameplay script. **PASS**
- `TestHomePage404`: GET /nonexistent → 404. **PASS**
- `TestMove_AfterWin`: Moves rejected after win, state preserved. **PASS**
- `TestPlayGameDraw`: Terminal draw detection. **PASS**

### `go vet ./...`
- No issues (exit 0)

### `bash /inputs/resources/team/scripts/lint.sh`
- golangci-lint v2.14.0 with errcheck, govet, ineffassign, staticcheck, unused enabled
- 0 issues (exit 0)

## Scope boundary check

| File | Status | Notes |
|------|--------|-------|
| `index.html` | Changed (expected) | Inline `<style>` removed, `<link rel="stylesheet" href="style.css">` added |
| `style.css` | New (expected) | Restyled with depth cues and coherent palette |
| `server.go` | Changed (expected) | `//go:embed index.html style.css`; `serveEmbeddedFile`; `newServerMux` factory |
| `server_test.go` | Changed (expected) | `setupServer` repointed; new asset-serving tests added |
| `main.go` | Unchanged ✓ | Terminal mode, checkWin, isDraw — identical to baseline |
| `main_test.go` | Unchanged ✓ | All terminal tests pass as-is |
| `go.mod` | Unchanged ✓ | Still `go 1.21` |
| `README.md` | Unchanged ✓ | Present at baseline |
| `ttt` binary | Unchanged ✓ | MD5 a593fba4b61c74975ad0d543e479b9f4 — pre-existing binary untouched |
| Build artifacts | Not committed ✓ | Build output written to `/runtime/build/`, not `/workspace` |

## Compliance with PLAN decisions

- **Shared `serveEmbeddedFile` (not `http.FileServer`)**: Implemented correctly — single handler used by both `/` and `/style.css` routes.
- **Production mux factory `newServerMux`**: Extracted and used by both `startWebServer` and `setupServer` (via `httptest`).
- **Method discipline**: All routes enforce GET/POST as appropriate; POST /style.css returns 405 (tested).
- **No Go 1.22+ method-pattern mux syntax**: Routes use explicit method checks, consistent with Go 1.21 compatibility.
- **Focused asset-serving tests (required)**: `TestStyleCSS_ServedWithTextCSSContentType`, `TestStyleCSS_RejectsNonGet`, `TestHomePage_LinksStylesheetAndKeepsMarkup` — all present and passing.
- **No duplicated test-only mux**: `setupServer` deleted the placeholder-HTML duplicate mux, now exercises production handlers.

## Verdict

All SPEC acceptance criteria (AC-1 through AC-5) are satisfied. All PLAN stories (US-001 through US-003) are complete. All four declared quality gates pass. The implementation stays within the declared scope; no file outside `index.html`, `style.css`, `server.go`, `server_test.go` was modified. No new dependencies, no `http.FileServer`, no CDN assets, no gameplay or API changes.

VERDICT: PASS