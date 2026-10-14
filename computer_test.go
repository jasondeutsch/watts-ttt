package main

// Tests for US-002 (AC-4, AC-8) and US-003 (AC-5): the computer reply in
// computer mode through the production handlers. The unexported
// pickComputerMove seam is used only to reach a specific reachable board or to
// force a particular legal O pick; the default production selector is
// math/rand over the empty cells.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestComputerMove_FixtureA_RealRandomLegality (fixture A) runs the paired-move
// scenario many times over the real random source and asserts legality only:
// human X placed, exactly one computer O in an allowed empty cell, X to move,
// game not over. No particular random outcome is asserted.
func TestComputerMove_FixtureA_RealRandomLegality(t *testing.T) {
	allowed := map[int]bool{2: true, 4: true, 5: true, 6: true, 7: true, 8: true}
	for i := 0; i < 50; i++ {
		srv, gs := setupComputerGS(t)

		// Reach the reachable fixture board X={0}, O={3} by forcing the first
		// computer reply with the seam...
		gs.pickComputerMove = func(b board) int { return 3 }
		resp, body := makeMove(t, srv, 0)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("run %d: expected 200, got %d: %s", i, resp.StatusCode, body)
		}
		state := parseStateResponse(t, body)
		assertCounts(t, state, 1, 1)
		if state.Board[0][0] != "X" || state.Board[1][0] != "O" {
			t.Fatalf("run %d: unexpected fixture board %v", i, state.Board)
		}
		if state.CurrentPlayer != "X" || state.GameOver {
			t.Fatalf("run %d: expected X to move and game not over", i)
		}

		// ...and restore the real random source for the move under test.
		gs.pickComputerMove = defaultPickComputerMove
		resp, body = makeMove(t, srv, 1)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("run %d: expected 200, got %d: %s", i, resp.StatusCode, body)
		}
		state = parseStateResponse(t, body)
		if state.Board[0][0] != "X" || state.Board[0][1] != "X" {
			t.Fatalf("run %d: human X marks missing on %v", i, state.Board)
		}
		x, o := countMarks(state)
		if x != 2 || o != 2 {
			t.Fatalf("run %d: expected 2 X and 2 O, got %d X and %d O", i, x, o)
		}
		compO := -1
		for r := 0; r < 3; r++ {
			for c := 0; c < 3; c++ {
				idx := r*3 + c
				// The setup O occupies cell 3; find the new computer O elsewhere.
				if state.Board[r][c] == "O" && idx != 3 {
					compO = idx
				}
			}
		}
		if compO < 0 {
			t.Fatalf("run %d: expected exactly one new computer O besides the setup O at 3", i)
		}
		if !allowed[compO] {
			t.Fatalf("run %d: computer O placed at %d, expected one of {2,4,5,6,7,8}", i, compO)
		}
		if state.CurrentPlayer != "X" {
			t.Fatalf("run %d: expected X to move, got %s", i, state.CurrentPlayer)
		}
		if state.GameOver {
			t.Fatalf("run %d: expected game not over", i)
		}
		if state.Mode != modeComputer {
			t.Fatalf("run %d: expected mode %q, got %q", i, modeComputer, state.Mode)
		}
	}
}

// TestComputerMove_FixtureB_SeamTerminalBranches (fixture B) drives the same
// production branch to both outcomes deterministically through the handlers:
// a computer O win (pick 5, gameOver true, winner O) and a nonterminal pair
// (pick 6, X to move, not over). AC-4 terminal behavior + AC-8.
func TestComputerMove_FixtureB_SeamTerminalBranches(t *testing.T) {
	setupBoard := func(t *testing.T, srv *httptest.Server, gs *gameState) {
		t.Helper()
		gs.pickComputerMove = func(b board) int { return 3 }
		makeMove(t, srv, 0) // X{0}, O{3}
		gs.pickComputerMove = func(b board) int { return 4 }
		_, preBody := makeMove(t, srv, 2) // X{0,2}, O{3,4}
		pre := parseStateResponse(t, preBody)
		assertInvariant(t, pre)
		assertCounts(t, pre, 2, 2)
	}

	t.Run("computer win with seam pick 5", func(t *testing.T) {
		srv, gs := setupComputerGS(t)
		setupBoard(t, srv, gs)
		gs.pickComputerMove = func(b board) int { return 5 }
		resp, body := makeMove(t, srv, 8) // O{3,4,5} completes the middle row
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}
		state := parseStateResponse(t, body)
		if !state.GameOver {
			t.Fatal("expected gameOver true after the computer's winning move")
		}
		if state.Winner != "O" {
			t.Errorf("expected winner O, got %q", state.Winner)
		}
		assertCounts(t, state, 3, 3)
		if state.Mode != modeComputer {
			t.Errorf("expected mode %q, got %q", modeComputer, state.Mode)
		}
	})

	t.Run("nonterminal pair with seam pick 6", func(t *testing.T) {
		srv, gs := setupComputerGS(t)
		setupBoard(t, srv, gs)
		gs.pickComputerMove = func(b board) int { return 6 }
		resp, body := makeMove(t, srv, 8) // O{3,4,6}: no line, board not full
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}
		state := parseStateResponse(t, body)
		assertInvariant(t, state)
		assertCounts(t, state, 3, 3)
		if state.CurrentPlayer != "X" {
			t.Errorf("expected X to move after the pair, got %s", state.CurrentPlayer)
		}
		if state.GameOver {
			t.Error("expected game not over for a nonterminal pair")
		}
		if state.Winner != "" {
			t.Errorf("expected no winner, got %q", state.Winner)
		}
		if state.Mode != modeComputer {
			t.Errorf("expected mode %q, got %q", modeComputer, state.Mode)
		}
	})
}

// TestComputerMove_FixtureC_HumanWinNoComputerMove (fixture C, AC-5): the
// human's winning move ends the game immediately; O count stays 2.
func TestComputerMove_FixtureC_HumanWinNoComputerMove(t *testing.T) {
	srv, gs := setupComputerGS(t)

	gs.pickComputerMove = func(b board) int { return 3 }
	makeMove(t, srv, 0) // X{0}, O{3}
	gs.pickComputerMove = func(b board) int { return 4 }
	_, preBody := makeMove(t, srv, 1) // X{0,1}, O{3,4}
	pre := parseStateResponse(t, preBody)
	assertInvariant(t, pre)
	assertCounts(t, pre, 2, 2)

	resp, body := makeMove(t, srv, 2) // X wins row 0-1-2
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	state := parseStateResponse(t, body)
	if !state.GameOver || state.Winner != "X" {
		t.Fatalf("expected an X win, got gameOver=%v winner=%q", state.GameOver, state.Winner)
	}
	// Exactly 3 X and 2 O: no computer O followed the winning human move.
	assertCounts(t, state, 3, 2)
	if state.Mode != modeComputer {
		t.Errorf("expected mode %q, got %q", modeComputer, state.Mode)
	}
}

// TestComputerMove_FixtureD_HumanFillsBoardDraw (fixture D, AC-5): the human's
// board-filling move ends the game as a draw; O count stays 4.
func TestComputerMove_FixtureD_HumanFillsBoardDraw(t *testing.T) {
	srv, gs := setupComputerGS(t)

	setup := []struct {
		human int
		comp  int
	}{
		{0, 1}, {2, 3}, {5, 4}, {6, 8},
	}
	var pre stateResponse
	for _, s := range setup {
		comp := s.comp // go 1.21 loop semantics: must not capture the loop variable
		gs.pickComputerMove = func(b board) int { return comp }
		_, body := makeMove(t, srv, s.human)
		pre = parseStateResponse(t, body)
	}
	// Reachable pre-move board: X{0,2,5,6}, O{1,3,4,8} — equal counts, X to move.
	assertInvariant(t, pre)
	assertCounts(t, pre, 4, 4)

	resp, body := makeMove(t, srv, 7) // X fills the board: X{0,2,5,6,7}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	state := parseStateResponse(t, body)
	if !state.GameOver || state.Winner != "" {
		t.Fatalf("expected a draw, got gameOver=%v winner=%q", state.GameOver, state.Winner)
	}
	// Exactly 5 X and 4 O: no computer O followed the draw-filling move.
	assertCounts(t, state, 5, 4)
	if state.Mode != modeComputer {
		t.Errorf("expected mode %q, got %q", modeComputer, state.Mode)
	}
}

// TestComputerMove_FixtureE_InvalidMovesNoComputerMove (fixture E, AC-5):
// occupied, out-of-range, and malformed-JSON moves each fail exactly as today
// and leave the state bit-for-bit unchanged, with no computer reply even
// though a live O threat exists.
func TestComputerMove_FixtureE_InvalidMovesNoComputerMove(t *testing.T) {
	srv, gs := setupComputerGS(t)

	gs.pickComputerMove = func(b board) int { return 3 }
	makeMove(t, srv, 0) // X{0}, O{3}
	gs.pickComputerMove = func(b board) int { return 4 }
	makeMove(t, srv, 1) // X{0,1}, O{3,4}; live O threat on cell 5

	_, preBody := getState(t, srv)
	pre := parseStateResponse(t, preBody)
	assertInvariant(t, pre)
	assertCounts(t, pre, 2, 2)

	// Occupied cell: 400 with today's state-response body, state unchanged.
	resp, body := makeMove(t, srv, 0)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("occupied cell: expected 400, got %d", resp.StatusCode)
	}
	if st := parseStateResponse(t, body); st.CurrentPlayer != "X" {
		t.Errorf("occupied cell: expected X still to move, got %s", st.CurrentPlayer)
	}

	// Out-of-range cell: 400 with the exact existing error message.
	resp, body = makeMove(t, srv, 99)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("out-of-range cell: expected 400, got %d", resp.StatusCode)
	}
	wantErr := fmt.Sprintf("cell %d is out of range (0-8)", 99)
	if er := parseErrorResponse(t, body); er.Error != wantErr {
		t.Errorf("out-of-range cell: expected %q, got %q", wantErr, er.Error)
	}

	// Malformed JSON: 400 with the exact existing error message.
	if resp := postMoveRaw(t, srv, "not json"); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("malformed JSON: expected 400, got %d", resp.StatusCode)
	}

	// The state is bit-for-bit unchanged by all invalid moves.
	_, afterBody := getState(t, srv)
	if afterBody != preBody {
		t.Error("state changed after invalid moves in computer mode (bit-for-bit comparison)")
	}
	after := parseStateResponse(t, afterBody)
	assertCounts(t, after, 2, 2)
	if after.CurrentPlayer != "X" || after.GameOver {
		t.Errorf("expected X to move and game not over, got %s gameOver=%v", after.CurrentPlayer, after.GameOver)
	}
	if after.Mode != modeComputer {
		t.Errorf("expected mode %q, got %q", modeComputer, after.Mode)
	}
}

func TestPostMove_NonPost(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/move")
	if err != nil {
		t.Fatalf("GET /move failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}
