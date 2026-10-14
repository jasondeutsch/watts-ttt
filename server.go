package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
)

//go:embed index.html style.css
var embeddedFiles embed.FS

// Game modes. The browser offers exactly these two values.
const (
	modeTwoPlayer = "two-player"
	modeComputer  = "computer"
)

// gameState holds the authoritative game state, protected by a mutex.
type gameState struct {
	mu            sync.Mutex
	board         board
	currentPlayer rune
	winner        rune
	gameOver      bool
	mode          string
	// pickComputerMove selects the computer's O cell in computer mode. It is a
	// test seam: production uses defaultPickComputerMove (uniform random via
	// math/rand); tests may replace it with a deterministic selector to force
	// particular outcomes through the same production handlers.
	pickComputerMove func(b board) int
}

// stateResponse is the JSON response for game state.
type stateResponse struct {
	Board         [][]string `json:"board"`
	CurrentPlayer string     `json:"currentPlayer"`
	Winner        string     `json:"winner"`
	GameOver      bool       `json:"gameOver"`
	Mode          string     `json:"mode"`
}

// ErrorResponse is an error response body.
type ErrorResponse struct {
	Error string `json:"error"`
}

// moveRequest is the JSON body for a move.
type moveRequest struct {
	Cell *int `json:"cell"`
}

// modeRequest is the JSON body for a mode change.
type modeRequest struct {
	Mode *string `json:"mode"`
}

// defaultPickComputerMove returns a uniformly random empty cell index using
// math/rand from the Go standard library only (AC-8). It must only be called
// when the board is not full.
func defaultPickComputerMove(b board) int {
	emptyCells := make([]int, 0, 9)
	for i, c := range b {
		if c == empty {
			emptyCells = append(emptyCells, i)
		}
	}
	return emptyCells[rand.Intn(len(emptyCells))]
}

// newGameState creates a new game state with an empty board in two-player mode.
func newGameState() *gameState {
	return &gameState{
		board:            newBoard(),
		currentPlayer:    playerX,
		mode:             modeTwoPlayer,
		pickComputerMove: defaultPickComputerMove,
	}
}

// toStateResponse converts the current game state to a JSON-serializable response.
func (gs *gameState) toStateResponse() stateResponse {
	board := make([][]string, 3)
	for i := 0; i < 3; i++ {
		board[i] = make([]string, 3)
		for j := 0; j < 3; j++ {
			cell := gs.board[i*3+j]
			if cell == empty {
				board[i][j] = " "
			} else {
				board[i][j] = string(cell)
			}
		}
	}
	currentPlayer := string(gs.currentPlayer)
	winner := ""
	if gs.winner != 0 {
		winner = string(gs.winner)
	}
	return stateResponse{
		Board:         board,
		CurrentPlayer: currentPlayer,
		Winner:        winner,
		GameOver:      gs.gameOver,
		Mode:          gs.mode,
	}
}

// serveEmbeddedFile returns a handler that serves a single embedded file
// with the given Content-Type.
func serveEmbeddedFile(name, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := embeddedFiles.ReadFile(name)
		if err != nil {
			log.Printf("error reading embedded %s: %v", name, err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}

// newServerMux builds the production HTTP mux for the given game state.
// It exists as a factory so tests and the running server exercise identical
// handlers.
func newServerMux(gs *gameState) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		serveEmbeddedFile("index.html", "text/html; charset=utf-8")(w, r)
	})

	mux.HandleFunc("/style.css", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		serveEmbeddedFile("style.css", "text/css")(w, r)
	})

	mux.HandleFunc("/state", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
			return
		}
		gs.mu.Lock()
		resp := gs.toStateResponse()
		gs.mu.Unlock()
		writeJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("/move", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
			return
		}

		var req moveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON body"})
			return
		}

		if req.Cell == nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "missing 'cell' field"})
			return
		}

		cell := *req.Cell

		gs.mu.Lock()
		defer gs.mu.Unlock()

		// Check if game is over
		if gs.gameOver {
			writeJSON(w, http.StatusBadRequest, gs.toStateResponse())
			return
		}

		// Validate cell range (0-8)
		if cell < 0 || cell > 8 {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("cell %d is out of range (0-8)", cell)})
			return
		}

		// Check if cell is occupied
		if gs.board[cell] != empty {
			writeJSON(w, http.StatusBadRequest, gs.toStateResponse())
			return
		}

		// Place the mark
		gs.board[cell] = gs.currentPlayer

		// Check for win or draw
		if winner, won := checkWin(gs.board); won {
			gs.winner = winner
			gs.gameOver = true
			writeJSON(w, http.StatusOK, gs.toStateResponse())
			return
		}
		if isDraw(gs.board) {
			gs.gameOver = true
			writeJSON(w, http.StatusOK, gs.toStateResponse())
			return
		}

		if gs.mode == modeComputer {
			// Exactly one automatic computer O move per valid nonterminal human
			// move (AC-4). The human gave the game over at this point, so there is
			// at least one empty cell. The human is always X and moves first, so
			// currentPlayer stays X for the human's next turn.
			compCell := gs.pickComputerMove(gs.board)
			gs.board[compCell] = playerO
			if winner, won := checkWin(gs.board); won {
				gs.winner = winner
				gs.gameOver = true
				writeJSON(w, http.StatusOK, gs.toStateResponse())
				return
			}
			if isDraw(gs.board) {
				gs.gameOver = true
				writeJSON(w, http.StatusOK, gs.toStateResponse())
				return
			}
			writeJSON(w, http.StatusOK, gs.toStateResponse())
			return
		}

		// Toggle player
		if gs.currentPlayer == playerX {
			gs.currentPlayer = playerO
		} else {
			gs.currentPlayer = playerX
		}

		writeJSON(w, http.StatusOK, gs.toStateResponse())
	})

	mux.HandleFunc("/mode", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
			return
		}

		var req modeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid mode"})
			return
		}
		if req.Mode == nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid mode"})
			return
		}
		mode := *req.Mode
		if mode != modeTwoPlayer && mode != modeComputer {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid mode"})
			return
		}

		gs.mu.Lock()
		defer gs.mu.Unlock()

		if mode == gs.mode {
			// Selecting the mode that is already active has no effect.
			writeJSON(w, http.StatusOK, gs.toStateResponse())
			return
		}

		// Selecting a different mode starts a fresh game in that mode (AC-2).
		gs.mode = mode
		gs.board = newBoard()
		gs.currentPlayer = playerX
		gs.winner = 0
		gs.gameOver = false
		writeJSON(w, http.StatusOK, gs.toStateResponse())
	})

	mux.HandleFunc("/reset", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
			return
		}

		gs.mu.Lock()
		gs.board = newBoard()
		gs.currentPlayer = playerX
		gs.winner = 0
		gs.gameOver = false
		resp := gs.toStateResponse()
		gs.mu.Unlock()

		// The active mode is preserved across reset (AC-3): gs.mode is untouched.
		writeJSON(w, http.StatusOK, resp)
	})

	return mux
}

// startWebServer starts the HTTP server on the given address.
func startWebServer(addr string) {
	gs := newGameState()

	// Wrap with logging middleware
	handler := loggingMiddleware(newServerMux(gs))

	log.Printf("Starting web server on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// loggingMiddleware logs each request with method, path, and status code.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lrw, r)
		log.Printf("%s %s %d", r.Method, r.URL.Path, lrw.statusCode)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// wirteJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
