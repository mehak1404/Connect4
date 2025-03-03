// api/handlers.go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	"connect4/db"
	"connect4/games"
)

// Error response structure
type ErrorResponse struct {
	Error string `json:"error"`
}

// Response helpers
func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, ErrorResponse{Error: message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"Internal Server Error"}`))
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

// RegisterGameConnection registers a websocket connection for a game
func RegisterGameConnection(gameID string, conn *websocket.Conn) {
	db.RegisterGameConnection(gameID, conn)
}

// Player handlers
// GetPlayers returns all players
func GetPlayers(w http.ResponseWriter, r *http.Request) {
	players, err := db.ListPlayers()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error retrieving players")
		return
	}
	
	respondWithJSON(w, http.StatusOK, players)
}

// CreatePlayer creates a new player
func CreatePlayer(w http.ResponseWriter, r *http.Request) {
	var player games.Player
	
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&player); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()
	
	if player.Username == "" {
		respondWithError(w, http.StatusBadRequest, "Username is required")
		return
	}
	
	if err := db.CreatePlayer(&player); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	respondWithJSON(w, http.StatusCreated, player)
}

// GetPlayer returns a specific player
func GetPlayer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	playerID := vars["id"]
	
	player, err := db.GetPlayer(playerID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Player not found")
		return
	}
	
	respondWithJSON(w, http.StatusOK, player)
}

// GetLeaderboard returns the player leaderboard
func GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 10 // Default limit
	
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid limit parameter")
			return
		}
	}
	
	leaderboard, err := db.GetLeaderboard(limit)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error retrieving leaderboard")
		return
	}
	
	respondWithJSON(w, http.StatusOK, leaderboard)
}

// Game handlers
// CreateGame creates a new game
func CreateGame(w http.ResponseWriter, r *http.Request) {
	var requestData struct {
		GameType  games.GameType `json:"gameType"`
		Player1ID string        `json:"player1Id"`
		Player2ID string        `json:"player2Id,omitempty"`
	}
	
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&requestData); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()
	
	// Validate game type
	if requestData.GameType != games.SinglePlayer && 
	   requestData.GameType != games.LocalMultiplayer && 
	   requestData.GameType != games.OnlineMultiplayer {
		respondWithError(w, http.StatusBadRequest, "Invalid game type")
		return
	}
	
	// Set default player IDs for single player mode
	if requestData.GameType == games.SinglePlayer && requestData.Player2ID == "" {
		requestData.Player2ID = "bot"
	}
	
	// Make sure player IDs are provided for multiplayer
	if requestData.GameType == games.OnlineMultiplayer && 
	  (requestData.Player1ID == "" || requestData.Player2ID == "") {
		respondWithError(w, http.StatusBadRequest, "Both player IDs required for online multiplayer")
		return
	}
	
	// Create the game
	newGame := games.NewGame(requestData.GameType, requestData.Player1ID, requestData.Player2ID)
	
	// Start the game immediately
	newGame.Status = games.StatusActive
	
	// Save the game
	if err := db.CreateGame(newGame); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating game")
		return
	}
	
	respondWithJSON(w, http.StatusCreated, newGame)
}

// GetGame returns a specific game
func GetGame(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]
	
	game, err := db.GetGame(gameID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Game not found")
		return
	}
	
	respondWithJSON(w, http.StatusOK, game)
}

// NOTE : we have to save the players in the game, not their id , or we could save the bot for each game
// MakeMove makes a move in a game
func MakeMove(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]
	
	var move games.Move
	
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&move); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()
	
	// Get the game
	currentGame, err := db.GetGame(gameID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Game not found")
		return
	}
	
	// Make the move
	if err := currentGame.MakeMove(move.PlayerID, move.Column); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	
	// Save the updated game
	if err := db.SaveGame(currentGame); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error saving game")
		return
	}
	
	// If game is against bot and it's bot's turn, make the bot move
	if currentGame.Status == games.StatusActive && 
	   ((currentGame.Player1ID == "bot" && currentGame.CurrentTurn == games.RedToken) || 
		(currentGame.Player2ID == "bot" && currentGame.CurrentTurn == games.YellowToken)) {
		
		// Get bot move
		botColumn := currentGame.Bot.GetNextMove(currentGame)
		botPlayerID := currentGame.Player1ID
		if currentGame.Player1ID != "bot" {
			botPlayerID = currentGame.Player2ID
		}
		
		// Apply bot move
		if err := currentGame.MakeMove(botPlayerID, botColumn); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Bot move error: "+err.Error())
			return
		}
		
		// Save the game state
		if err := db.SaveGame(currentGame); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error saving game after bot move")
			return
		}
	}
	
	// Broadcast game update to WebSocket clients
	db.BroadcastGameState(gameID, currentGame)
	
	// Return the updated game
	respondWithJSON(w, http.StatusOK, currentGame)
}
