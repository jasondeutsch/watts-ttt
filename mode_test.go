package main

// Tests for US-001: server mode state, lifecycle, reset semantics, and the
// additive Mode field (AC-1, AC-2, AC-3, AC-6, AC-7). All requests flow
// through the production handlers via newServerMux.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func postMode(t *testing.T, srv *httptest.Server, body string) (*http.Response, string) {
	t.Helper()
	resp, err := http.Post(srv.URL+"/mode", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /mode failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var sb strings.Builder
	_, _ = io.Copy(&sb, resp.Body)
	return resp, sb.String()
}

// postModeJSON posts a valid mode and returns the parsed fresh/current state.
func postModeJSON(t *testing.T, srv *httptest.Server, mode string) stateResponse {
	t.Helper()
	resp, body := postMode(t, srv, fmt.Sprintf(`{"mode":%q}`, mode))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for POST /mode %s, got %d: %s", mode, resp.StatusCode, body)
	}
	return parseStateResponse(t, body)
}

// postMoveRaw posts an arbitrary /move body and returns the response (closed).
func postMoveRaw(t *testing.T, srv *httptest.Server, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(srv.URL+"/move", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /move failed: %v", err)
	}
	_ = resp.Body.Close()
	return resp
}

// countMarks counts X and O marks on the response board.
func countMarks(state stateResponse) (x, o int) {
	for _, row := range state.Board {
		for _, cell := range row {
			switch strings.TrimSpace(cell) {
			case "X":
				x++
			case "O":
				o++
			}
		}
	}
	return x, o
}

func assertEmptyBoard(t *testing.T, state stateResponse) {
	t.Helper()
	for _, row := range state.Board {
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				t.Errorf("expected empty board, got %q", cell)
			}
		}
	}
}

// assertFreshGame asserts the shape of a freshly started game in wantMode.
func assertFreshGame(t *testing.T, state stateResponse, wantMode string) {
	t.Helper()
	if state.CurrentPlayer != "X" {
		t.Errorf("expected X to move, got %s", state.CurrentPlayer)
	}
	if state.Winner != "" {
		t.Errorf("expected no winner, got %q", state.Winner)
	}
	if state.GameOver {
		t.Error("expected gameOver false")
	}
	if state.Mode != wantMode {
		t.Errorf("expected mode %q, got %q", wantMode, state.Mode)
	}
	assertEmptyBoard(t, state)
}

func assertCounts(t *testing.T, state stateResponse, wantX, wantO int) {
	t.Helper()
	x, o := countMarks(state)
	if x != wantX || o != wantO {
		t.Errorf("expected %d X and %d O, got %d X and %d O", wantX, wantO, x, o)
	}
}

// assertInvariant checks the equal-counts invariant stated in the plan: every
// reachable pre-move board has X to move and equal X and O counts.
func assertInvariant(t *testing.T, state stateResponse) {
	t.Helper()
	if state.CurrentPlayer != "X" {
		t.Errorf("invariant: expected X to move, got %s", state.CurrentPlayer)
	}
	x, o := countMarks(state)
	if x != o {
		t.Errorf("invariant: expected |X| == |O|, got %d vs %d", x, o)
	}
}

// setupComputerGS returns a server already in computer mode plus the gameState
// it wraps, so tests can replace the unexported pickComputerMove seam.
func setupComputerGS(t *testing.T) (*httptest.Server, *gameState) {
	t.Helper()
	gs := newGameState()
	gs.mode = modeComputer
	srv := httptest.NewServer(newServerMux(gs))
	t.Cleanup(srv.Close)
	return srv, gs
}

func TestState_InitialModeIsTwoPlayer(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	resp, body := getState(t, srv)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	assertFreshGame(t, parseStateResponse(t, body), modeTwoPlayer)
}

func TestState_ModeFieldPresentThroughout(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	// Initial state.
	_, body := getState(t, srv)
	if st := parseStateResponse(t, body); st.Mode != modeTwoPlayer {
		t.Errorf("initial mode: expected %q, got %q", modeTwoPlayer, st.Mode)
	}

	// After a two-player move.
	makeMove(t, srv, 0)
	_, body = getState(t, srv)
	if st := parseStateResponse(t, body); st.Mode != modeTwoPlayer {
		t.Errorf("mode after move: expected %q, got %q", modeTwoPlayer, st.Mode)
	}

	// After a mode switch and a computer pair move.
	if st := postModeJSON(t, srv, modeComputer); st.Mode != modeComputer {
		t.Errorf("mode after switch: expected %q, got %q", modeComputer, st.Mode)
	}
	makeMove(t, srv, 0)
	_, body = getState(t, srv)
	if st := parseStateResponse(t, body); st.Mode != modeComputer {
		t.Errorf("mode after computer pair: expected %q, got %q", modeComputer, st.Mode)
	}

	// After a reset.
	postReset(t, srv)
	_, body = getState(t, srv)
	if st := parseStateResponse(t, body); st.Mode != modeComputer {
		t.Errorf("mode after reset: expected %q, got %q", modeComputer, st.Mode)
	}
}

func TestMode_SwitchStartsFreshGameMidGame(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	makeMove(t, srv, 0) // X
	makeMove(t, srv, 3) // O
	_, body := getState(t, srv)
	midGame := parseStateResponse(t, body)
	assertCounts(t, midGame, 1, 1)
	if midGame.CurrentPlayer != "X" {
		t.Errorf("expected X to move, got %s", midGame.CurrentPlayer)
	}

	state := postModeJSON(t, srv, modeComputer)
	assertFreshGame(t, state, modeComputer)
}

func TestMode_SwitchStartsFreshGameAfterGameOver(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	// Two-player X win: 0,3,1,4,2.
	makeMove(t, srv, 0)
	makeMove(t, srv, 3)
	makeMove(t, srv, 1)
	makeMove(t, srv, 4)
	resp, body := makeMove(t, srv, 2)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for the winning move, got %d", resp.StatusCode)
	}
	if st := parseStateResponse(t, body); !st.GameOver || st.Winner != "X" {
		t.Fatalf("setup failed to produce an X win: gameOver=%v winner=%q", st.GameOver, st.Winner)
	}

	// Switching out of a finished game starts a fresh game in the new mode.
	state := postModeJSON(t, srv, modeComputer)
	assertFreshGame(t, state, modeComputer)

	// And switching back is likewise a fresh game.
	state = postModeJSON(t, srv, modeTwoPlayer)
	assertFreshGame(t, state, modeTwoPlayer)
}

func TestMode_AlreadyActiveHasNoEffect(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	// Two-player: the active mode is already selected during a live game.
	makeMove(t, srv, 0)
	state := postModeJSON(t, srv, modeTwoPlayer)
	if state.Board[0][0] != "X" {
		t.Error("expected the board to stay untouched when re-selecting the active mode")
	}
	if state.CurrentPlayer != "O" {
		t.Errorf("expected O to move, got %s", state.CurrentPlayer)
	}
	if state.Mode != modeTwoPlayer {
		t.Errorf("expected mode %q, got %q", modeTwoPlayer, state.Mode)
	}

	// Computer: re-selecting the active mode keeps the board and turn.
	state = postModeJSON(t, srv, modeComputer)
	assertFreshGame(t, state, modeComputer)
	makeMove(t, srv, 4) // human X; the computer replies with one O.
	state = postModeJSON(t, srv, modeComputer)
	if state.Mode != modeComputer {
		t.Errorf("expected mode %q, got %q", modeComputer, state.Mode)
	}
	assertCounts(t, state, 1, 1)
	if state.CurrentPlayer != "X" {
		t.Errorf("expected X to move, got %s", state.CurrentPlayer)
	}
}

func TestMode_InvalidBodies(t *testing.T) {
	bodies := []string{`{}`, `{"mode":"solo"}`, `{"mode":123}`, "not json"}
	srv := setupServer()
	defer srv.Close()

	respM, _ := makeMove(t, srv, 0)
	if respM.StatusCode != http.StatusOK {
		t.Fatalf("setup move failed with %d", respM.StatusCode)
	}
	_, beforeBody := getState(t, srv)

	for _, body := range bodies {
		resp, respBody := postMode(t, srv, body)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("body %q: expected 400, got %d", body, resp.StatusCode)
		}
		var er ErrorResponse
		if err := json.Unmarshal([]byte(respBody), &er); err != nil {
			t.Errorf("body %q: error body is not JSON: %v", body, err)
		} else if er.Error != "invalid mode" {
			t.Errorf("body %q: expected error %q, got %q", body, "invalid mode", er.Error)
		}
		_, afterBody := getState(t, srv)
		if afterBody != beforeBody {
			t.Errorf("body %q: state changed after an invalid mode request", body)
		}
	}
}

func TestMode_NonPostMethod(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/mode")
	if err != nil {
		t.Fatalf("GET /mode failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
	var body strings.Builder
	_, _ = io.Copy(&body, resp.Body)
	var er ErrorResponse
	if err := json.Unmarshal([]byte(body.String()), &er); err != nil {
		t.Fatalf("error body is not JSON: %v", err)
	}
	if er.Error != "method not allowed" {
		t.Errorf("expected %q, got %q", "method not allowed", er.Error)
	}
}

func TestReset_PreservesModeTwoPlayer(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	makeMove(t, srv, 0)
	makeMove(t, srv, 3)
	resp, body := postReset(t, srv)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	assertFreshGame(t, parseStateResponse(t, body), modeTwoPlayer)
}

func TestReset_PreservesModeComputer(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	postModeJSON(t, srv, modeComputer)
	makeMove(t, srv, 0) // human X plus one computer O.
	resp, body := postReset(t, srv)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	assertFreshGame(t, parseStateResponse(t, body), modeComputer)
}

func TestReset_PreservesModeComputerAfterGameOver(t *testing.T) {
	srv, gs := setupComputerGS(t)

	// Human X win, computer replies forced via the seam: X 0,1,2 vs O 3,4.
	gs.pickComputerMove = func(b board) int { return 3 }
	makeMove(t, srv, 0)
	gs.pickComputerMove = func(b board) int { return 4 }
	makeMove(t, srv, 1)
	resp, body := makeMove(t, srv, 2)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for the winning move, got %d", resp.StatusCode)
	}
	if st := parseStateResponse(t, body); !st.GameOver || st.Winner != "X" {
		t.Fatalf("setup failed to produce an X win: gameOver=%v winner=%q", st.GameOver, st.Winner)
	}

	resp, body = postReset(t, srv)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	assertFreshGame(t, parseStateResponse(t, body), modeComputer)
}

// TestHomePage_IncludesModeSelectorAndGuard is a static-asset regression check
// (US-004): the served page offers exactly the two documented mode controls and
// the script wires them to POST /mode and state.mode with the in-flight guard.
// It asserts served bytes only, never browser behavior.
func TestHomePage_IncludesModeSelectorAndGuard(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading / body: %v", err)
	}
	html := string(body)
	for _, expected := range []string{
		`id="mode-selector"`,
		`id="mode-display"`,
		`name="mode" value="two-player"`,
		`name="mode" value="computer"`,
		`>Two players</span>`,
		`>Computer</span>`,
		`'/mode'`,
		`state.mode`,
		`moveInFlight`,
	} {
		if !strings.Contains(html, expected) {
			t.Errorf("served HTML is missing %q", expected)
		}
	}
	if strings.Contains(html, "<style") {
		t.Error("served HTML still contains an inline <style> block")
	}
}

// TestTwoPlayer_NeverComputes guards AC-6: in two-player mode the server still
// alternates X and O with no automatic computer move, byte-identical to today.
func TestTwoPlayer_NeverComputes(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	makeMove(t, srv, 0)
	_, body := getState(t, srv)
	st := parseStateResponse(t, body)
	assertCounts(t, st, 1, 0)
	if st.CurrentPlayer != "O" {
		t.Errorf("expected O to move after one two-player move, got %s", st.CurrentPlayer)
	}
	if st.Mode != modeTwoPlayer {
		t.Errorf("expected mode %q, got %q", modeTwoPlayer, st.Mode)
	}

	makeMove(t, srv, 1)
	_, body = getState(t, srv)
	st = parseStateResponse(t, body)
	assertCounts(t, st, 1, 1)
	if st.CurrentPlayer != "X" {
		t.Errorf("expected X to move after two two-player moves, got %s", st.CurrentPlayer)
	}
}
