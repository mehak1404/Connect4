package games

import (
	"errors"
	"time"
)
type GameStatus string

type GameType string

type Game struct {
	ID           string    `json:"id"`
	Type         GameType  `json:"type"`
	Board        [][]int   `json:"board"`
	CurrentTurn  int       `json:"currentTurn"`
	Player1ID    string    `json:"player1Id"`
	Player2ID    string    `json:"player2Id"` // Could be "bot" for single player
	WinnerID     string    `json:"winnerId,omitempty"`
	Status       GameStatus `json:"status"`
	LastMoveTime time.Time `json:"lastMoveTime"`
	CreatedAt    time.Time `json:"createdAt"`
	Bot        *BotPlayer 
}

type Player struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Wins     int    `json:"wins"`
	Losses   int    `json:"losses"`
	CreatedAt time.Time `json:"createdAt"`
}

// func CreatePlayer()

type Move struct {
	PlayerID string `json:"playerId"`
	Column   int    `json:"column"`
}


func NewPlayer(Username string) * Player {
	return &Player{
		ID:        generatePlayerID(),
		Username:  Username,
		Wins:      0,
		Losses:    0,
		CreatedAt: time.Now(),
	}
	
}

func NewBoard() [][]int {
	board := make([][]int, BoardHeight)
	for i := range board {
		board[i] = make([]int, BoardWidth)
	}
	return board
}
// NewGame creates a new game with an empty board
func NewGame(gameType GameType, player1ID, player2ID string) *Game {
	// Initialize empty board
	board := NewBoard()

	game := &Game{
		ID:          generateGameID(),
		Type:        gameType,
		Board:       board,
		CurrentTurn: RedToken, // Red always starts
		Player1ID:   player1ID,
		Player2ID:   player2ID,
		Status:      StatusWaiting,
		CreatedAt:   time.Now(),
		
	}
	// Initialize a bot if one of the players is a bot
    if player1ID == "bot" {
        game.Bot = NewBotPlayer(player1ID, RedToken)
    } else if player2ID == "bot" {
        game.Bot = NewBotPlayer(player2ID, YellowToken)
    }

	return game
}

// MakeMove attempts to drop a token in the specified column
func (g *Game) MakeMove(playerID string, column int) error {
	// Check if it's this player's turn
	if g.Status != StatusActive {
		return errors.New("game is not active")
	}
	
	// Determine which token this player uses
	var playerToken int
	if playerID == g.Player1ID {
		playerToken = RedToken
	} else if playerID == g.Player2ID {
		playerToken = YellowToken
	} else {
		return errors.New("player is not in this game")
	}
	
	// Check if it's this player's turn
	if playerToken != g.CurrentTurn {
		return errors.New("not your turn")
	}
	
	// Check if column is valid
	if column < 0 || column >= BoardWidth {
		return errors.New("invalid column")
	}
	
	// Find the bottom-most empty cell in the column
	row := -1
	for r := BoardHeight - 1; r >= 0; r-- {
		if g.Board[r][column] == EmptyCell {
			row = r
			break
		}
	}
	
	if row == -1 {
		return errors.New("column is full")
	}
	
	// Place the token
	g.Board[row][column] = playerToken
	
	// Check for win condition
	if g.checkWinCondition(row, column, playerToken) {
		g.Status = StatusFinished
		if playerToken == RedToken {
			g.WinnerID = g.Player1ID
		} else {
			g.WinnerID = g.Player2ID
		}
		return nil
	}
	
	// Check for draw
	if g.isBoardFull() {
		g.Status = StatusFinished
		return nil
	}
	
	// Switch turns
	if g.CurrentTurn == RedToken {
		g.CurrentTurn = YellowToken
	} else {
		g.CurrentTurn = RedToken
	}
	
	g.LastMoveTime = time.Now()
	
	return nil
}

// isBoardFull checks if the board is completely filled
func (g *Game) isBoardFull() bool {
	for col := 0; col < BoardWidth; col++ {
		if g.Board[0][col] == EmptyCell {
			return false
		}
	}
	return true
}

// checkWinCondition checks if the last move resulted in a win
func (g *Game) checkWinCondition(row, col, playerToken int) bool {
	// Check horizontal
	if g.countConsecutive(row, col, 0, 1, playerToken) + g.countConsecutive(row, col, 0, -1, playerToken) - 1 >= 4 {
		return true
	}
	
	// Check vertical
	if g.countConsecutive(row, col, 1, 0, playerToken) + g.countConsecutive(row, col, -1, 0, playerToken) - 1 >= 4 {
		return true
	}
	
	// Check diagonal (/)
	if g.countConsecutive(row, col, -1, 1, playerToken) + g.countConsecutive(row, col, 1, -1, playerToken) - 1 >= 4 {
		return true
	}
	
	// Check diagonal (\)
	if g.countConsecutive(row, col, -1, -1, playerToken) + g.countConsecutive(row, col, 1, 1, playerToken) - 1 >= 4 {
		return true
	}
	
	return false
}

// countConsecutive counts consecutive tokens in a direction
func (g *Game) countConsecutive(row, col, rowDelta, colDelta, playerToken int) int {
	count := 0
	r, c := row, col
	
	for r >= 0 && r < BoardHeight && c >= 0 && c < BoardWidth && g.Board[r][c] == playerToken {
		count++
		r += rowDelta
		c += colDelta
	}
	
	return count
}

// Helper functions
func generateGameID() string {
	// In a real implementation, use a proper UUID library
	return "game_" + time.Now().Format("20060102150405")
}

func generatePlayerID() string {
	return "player_" + time.Now().Format("20060102150405")
}