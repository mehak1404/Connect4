package games

import (
	"math"
	"time"
)

// Constants for board evaluation
const (
	WinScore    = 1000000  // Score for a winning position
	ThreeInRow  = 1000     // Score for three in a row
	TwoInRow    = 10       // Score for two in a row
	OneInRow    = 1        // Score for a single piece
	MaxDepth    = 7        // Maximum depth for minimax
	TimeLimit   = 980      // Time limit in milliseconds
)

// BotPlayer implements an optimized minimax bot with alpha-beta pruning and dynamic programming
type BotPlayer struct {
	PlayerID     string
	PlayerToken  int
	OpponentToken int
	TransTable   map[string]int   // Transposition table for dynamic programming
	NodesExplored int             // For statistics
	StartTime    time.Time        // For time management
}

// NewBotPlayer creates a new bot player
func NewBotPlayer(playerID string, playerToken int) *BotPlayer {
	opponentToken := RedToken
	if playerToken == RedToken {
		opponentToken = YellowToken
	}
	
	return &BotPlayer{
		PlayerID:     playerID,
		PlayerToken:  playerToken,
		OpponentToken: opponentToken,
		TransTable:   make(map[string]int),
	}
}

// GetNextMove returns the best move for the bot
func (bot *BotPlayer) GetNextMove(game *Game) int {
	bot.StartTime = time.Now()
	bot.NodesExplored = 0
	bot.TransTable = make(map[string]int)
	
	// Count empty slots to determine search depth
	emptySlots := bot.countEmptySlots(game.Board)
	depthLimit := MaxDepth
	
	// Adjust depth based on number of empty slots
	if emptySlots < (BoardHeight*BoardWidth)/3 {
		depthLimit = 9 // Go deeper in endgame
	}
	
	bestScore := math.MinInt32
	bestMove := -1
	
	// Try each column
	for col := 0; col < BoardWidth; col++ {
		if bot.isValidMove(game.Board, col) {
			// Make a copy of the board
			boardCopy := bot.copyBoard(game.Board)
			
			// Simulate the move
			row := bot.getNextAvailableRow(boardCopy, col)
			boardCopy[row][col] = bot.PlayerToken
			
			// If this is a winning move, return it immediately
			if bot.checkWin(boardCopy, row, col, bot.PlayerToken) {
				return col
			}
			
			// Evaluate the move
			score := bot.minimax(boardCopy, depthLimit-1, math.MinInt32, math.MaxInt32, false)
			
			// Check if time is running out
			if time.Since(bot.StartTime).Milliseconds() > TimeLimit {
				// If we're running out of time, use the best move found so far
				if bestMove == -1 {
					bestMove = col // At least return a valid move
				}
				break
			}
			
			if score > bestScore || (score == bestScore && col == BoardWidth/2) {
				bestScore = score
				bestMove = col
			}
		}
	}
	
	// Fallback to first valid move if no best move found
	if bestMove == -1 {
		for col := 0; col < BoardWidth; col++ {
			if bot.isValidMove(game.Board, col) {
				bestMove = col
				break
			}
		}
	}
	
	return bestMove
}

// minimax implements the minimax algorithm with alpha-beta pruning
func (bot *BotPlayer) minimax(board [][]int, depth int, alpha int, beta int, maximizingPlayer bool) int {
	// Check if time limit is approaching
	if time.Since(bot.StartTime).Milliseconds() > TimeLimit {
		return 0 // Return neutral score if we're out of time
	}
	
	bot.NodesExplored++
	
	// Check for terminal states
	boardKey := bot.boardToString(board)
	if cachedScore, found := bot.TransTable[boardKey]; found {
		return cachedScore
	}
	
	// Check if the board is full
	if bot.isBoardFull(board) {
		return 0 // Draw
	}
	
	// Check if depth limit reached
	if depth == 0 {
		score := bot.evaluateBoard(board)
		bot.TransTable[boardKey] = score
		return score
	}
	
	if maximizingPlayer {
		maxScore := math.MinInt32
		
		// Try each column
		for col := 0; col < BoardWidth; col++ {
			if bot.isValidMove(board, col) {
				// Make a copy of the board
				boardCopy := bot.copyBoard(board)
				
				// Simulate the move
				row := bot.getNextAvailableRow(boardCopy, col)
				boardCopy[row][col] = bot.PlayerToken
				
				// Check for win
				if bot.checkWin(boardCopy, row, col, bot.PlayerToken) {
					return WinScore
				}
				
				score := bot.minimax(boardCopy, depth-1, alpha, beta, false)
				maxScore = max(maxScore, score)
				alpha = max(alpha, maxScore)
				
				if beta <= alpha {
					break // Beta cutoff
				}
			}
		}
		
		bot.TransTable[boardKey] = maxScore
		return maxScore
	} else {
		minScore := math.MaxInt32
		
		// Try each column
		for col := 0; col < BoardWidth; col++ {
			if bot.isValidMove(board, col) {
				// Make a copy of the board
				boardCopy := bot.copyBoard(board)
				
				// Simulate the move
				row := bot.getNextAvailableRow(boardCopy, col)
				boardCopy[row][col] = bot.OpponentToken
				
				// Check for win
				if bot.checkWin(boardCopy, row, col, bot.OpponentToken) {
					return -WinScore
				}
				
				score := bot.minimax(boardCopy, depth-1, alpha, beta, true)
				minScore = min(minScore, score)
				beta = min(beta, minScore)
				
				if beta <= alpha {
					break // Alpha cutoff
				}
			}
		}
		
		bot.TransTable[boardKey] = minScore
		return minScore
	}
}

// evaluateBoard evaluates the current board position
func (bot *BotPlayer) evaluateBoard(board [][]int) int {
	score := 0
	
	// Evaluate horizontal windows
	for row := 0; row < BoardHeight; row++ {
		for col := 0; col <= BoardWidth-4; col++ {
			window := []int{board[row][col], board[row][col+1], board[row][col+2], board[row][col+3]}
			score += bot.evaluateWindow(window)
		}
	}
	
	// Evaluate vertical windows
	for col := 0; col < BoardWidth; col++ {
		for row := 0; row <= BoardHeight-4; row++ {
			window := []int{board[row][col], board[row+1][col], board[row+2][col], board[row+3][col]}
			score += bot.evaluateWindow(window)
		}
	}
	
	// Evaluate diagonal windows (/)
	for row := 3; row < BoardHeight; row++ {
		for col := 0; col <= BoardWidth-4; col++ {
			window := []int{board[row][col], board[row-1][col+1], board[row-2][col+2], board[row-3][col+3]}
			score += bot.evaluateWindow(window)
		}
	}
	
	// Evaluate diagonal windows (\)
	for row := 0; row <= BoardHeight-4; row++ {
		for col := 0; col <= BoardWidth-4; col++ {
			window := []int{board[row][col], board[row+1][col+1], board[row+2][col+2], board[row+3][col+3]}
			score += bot.evaluateWindow(window)
		}
	}
	
	// Center column preference
	centerCol := BoardWidth / 2
	centerCount := 0
	for row := 0; row < BoardHeight; row++ {
		if board[row][centerCol] == bot.PlayerToken {
			centerCount++
		}
	}
	score += centerCount * 3
	
	return score
}

// evaluateWindow evaluates a window of 4 positions
func (bot *BotPlayer) evaluateWindow(window []int) int {
	playerCount := 0
	opponentCount := 0
	emptyCount := 0
	
	for _, cell := range window {
		if cell == bot.PlayerToken {
			playerCount++
		} else if cell == bot.OpponentToken {
			opponentCount++
		} else {
			emptyCount++
		}
	}
	
	// Score the window
	if playerCount == 4 {
		return WinScore
	} else if playerCount == 3 && emptyCount == 1 {
		return ThreeInRow
	} else if playerCount == 2 && emptyCount == 2 {
		return TwoInRow
	} else if playerCount == 1 && emptyCount == 3 {
		return OneInRow
	}
	
	// Penalty for opponent threats
	if opponentCount == 3 && emptyCount == 1 {
		return -ThreeInRow * 2 // Prioritize blocking opponent wins
	} else if opponentCount == 2 && emptyCount == 2 {
		return -TwoInRow
	}
	
	return 0
}

// Helper functions
func (bot *BotPlayer) isValidMove(board [][]int, col int) bool {
	return col >= 0 && col < BoardWidth && board[0][col] == EmptyCell
}

func (bot *BotPlayer) getNextAvailableRow(board [][]int, col int) int {
	for row := BoardHeight - 1; row >= 0; row-- {
		if board[row][col] == EmptyCell {
			return row
		}
	}
	return -1 // Column is full
}

func (bot *BotPlayer) copyBoard(board [][]int) [][]int {
	newBoard := make([][]int, len(board))
	for i := range board {
		newBoard[i] = make([]int, len(board[i]))
		copy(newBoard[i], board[i])
	}
	return newBoard
}

func (bot *BotPlayer) checkWin(board [][]int, row, col, playerToken int) bool {
	// Check horizontal
	count := 1
	for c := col + 1; c < BoardWidth && board[row][c] == playerToken; c++ {
		count++
	}
	for c := col - 1; c >= 0 && board[row][c] == playerToken; c-- {
		count++
	}
	if count >= 4 {
		return true
	}
	
	// Check vertical
	count = 1
	for r := row + 1; r < BoardHeight && board[r][col] == playerToken; r++ {
		count++
	}
	for r := row - 1; r >= 0 && board[r][col] == playerToken; r-- {
		count++
	}
	if count >= 4 {
		return true
	}
	
	// Check diagonal (/)
	count = 1
	for i := 1; row - i >= 0 && col + i < BoardWidth && board[row-i][col+i] == playerToken; i++ {
		count++
	}
	for i := 1; row + i < BoardHeight && col - i >= 0 && board[row+i][col-i] == playerToken; i++ {
		count++
	}
	if count >= 4 {
		return true
	}
	
	// Check diagonal (\)
	count = 1
	for i := 1; row - i >= 0 && col - i >= 0 && board[row-i][col-i] == playerToken; i++ {
		count++
	}
	for i := 1; row + i < BoardHeight && col + i < BoardWidth && board[row+i][col+i] == playerToken; i++ {
		count++
	}
	if count >= 4 {
		return true
	}
	
	return false
}

func (bot *BotPlayer) isBoardFull(board [][]int) bool {
	for col := 0; col < BoardWidth; col++ {
		if board[0][col] == EmptyCell {
			return false
		}
	}
	return true
}

func (bot *BotPlayer) countEmptySlots(board [][]int) int {
	count := 0
	for row := 0; row < BoardHeight; row++ {
		for col := 0; col < BoardWidth; col++ {
			if board[row][col] == EmptyCell {
				count++
			}
		}
	}
	return count
}

func (bot *BotPlayer) boardToString(board [][]int) string {
	result := ""
	for row := 0; row < BoardHeight; row++ {
		for col := 0; col < BoardWidth; col++ {
			result += string(rune('0' + board[row][col]))
		}
	}
	return result
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// CreateBot creates a new bot player
func CreateBot(playerID string, playerToken int) *BotPlayer {
	return NewBotPlayer(playerID, playerToken)
}

// BotsNextMove calculates and returns the best move for the bot
func BotsNextMove(game *Game) int {
	playerToken := YellowToken
	if game.CurrentTurn == RedToken {
		playerToken = RedToken
	}
	
	bot := CreateBot("bot", playerToken)
	return bot.GetNextMove(game)
}