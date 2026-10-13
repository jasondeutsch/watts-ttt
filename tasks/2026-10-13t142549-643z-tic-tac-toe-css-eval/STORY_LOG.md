# Story log

Execution record for /workspace (Ralph iteration 1). DONE is appended only
after the declared quality gates pass from /workspace:
`build.sh`, `go test ./...`, `go vet ./...`, `lint.sh`.

DONE US-001 - Serve the page's styling from a separate embedded CSS file (2026-10-13)
- index.html: inline <style> removed, single <link rel="stylesheet" href="style.css"> in <head>; markup/IDs/classes/script unchanged.
- style.css: created at project root (initially verbatim baseline rules; restyled in US-002).
- server.go: //go:embed now covers index.html + style.css; shared serveEmbeddedFile(name, contentType) used by / and /style.css; newServerMux(gs) factory extracted and used by startWebServer.
- server_test.go: setupServer repointed at httptest.NewServer(newServerMux(newGameState())); placeholder-HTML duplicate mux deleted; focused tests added (GET /style.css -> 200 + text/css + exact embedded bytes; GET / -> 200 + text/html + link present + no inline <style> + documented IDs/classes and gameplay script preserved; POST /style.css -> 405).
- Gates: build.sh PASS, go test ./... PASS, go vet ./... PASS, lint.sh PASS.

DONE US-002 - Restyle the board with depth cues and a coherent palette (2026-10-13)
- Only style.css changed for presentation; index.html markup/IDs/classes/script untouched (asserted in TestHomePage_LinksStylesheetAndKeepsMarkup).
- Depth cues: layered shadows on board and cells, gradient-beveled raised tiles (inset highlight + inset bottom shade + outer drop shadows), dimensional panel border/radius; coherent slate-on-ivory palette with strong-contrast X (#1d4ed8) and O (#e11d48) marks with subtle glows.
- Responsive: aspect-ratio grid + max-width 420px container + 420px media query (smaller cells/fonts/padding); no page overflow at mobile or desktop.
- Accessibility: :focus-visible outline on cells, :focus outline on reset button; prefers-reduced-motion disables transitions/animations.
- Gates: build.sh PASS, go test ./... PASS, go vet ./... PASS, lint.sh PASS.

DONE US-003 - Verify gameplay, API schema, and terminal behavior are unchanged (2026-10-13)
- setupServer now exercises the real production handlers (newServerMux); /state, /move, /reset paths/methods/JSON bodies unchanged; existing server tests pass unchanged aside from setup replacement; main.go + main_test.go untouched and pass.
- /style.css is the only new public path (text/css); unknown paths return 404; POST /style.css -> 405.
- Scope: diff touches only index.html, style.css, server.go, server_test.go; main.go, game rules, JS gameplay, go.mod, README.md, and checked-in ttt binary untouched; no build artifacts committed (build.sh writes only under /runtime/build).
- End-to-end smoke against the built binary (GET / 200 text/html, GET /style.css 200 text/css, GET /nonexistent 404, POST /move 200 application/json) PASSED.
- Gates: build.sh PASS, go test ./... PASS, go vet ./... PASS, lint.sh PASS.