package db

import (
	"connect4/games"
	"encoding/json"
	"log"
	"sync"
	"time"
	"errors"
	"github.com/gorilla/websocket"
)

type MessageType string

// message type for websocket meesages
const (

	TypeGameState MessageType = "gameState"
	TypeMove MessageType = "move"
	TypeError MessageType = "error"
	TypeJoinGame MessageType = "joinGame"
	TypeResetRequest MessageType = "resetRequest"  // New: First player requests reset
    TypeResetConfirm MessageType = "resetConfirm"
	TypeResetGame MessageType = "resetGame"

)

// message going to have a type and paylaod
type Message struct {
	Type MessageType `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// err msg
type ErrorMessage struct {
	Error string `json:"error"`
}

// connection map

var (
	connections  = make(map[string][]*websocket.Conn)
	connMutex = &sync.Mutex{}
	
) 
var (
    playerConnections = make(map[string]*websocket.Conn)  
   
)

// Add these functions to manage player connections
func RegisterPlayerConnection(playerID string, conn *websocket.Conn) {
    playerMutex.Lock()
    defer playerMutex.Unlock()
    playerConnections[playerID] = conn
}

func GetPlayerConnection(playerID string) *websocket.Conn {
    playerMutex.Lock()
    defer playerMutex.Unlock()
    return playerConnections[playerID]
}

func RemovePlayerConnection(playerID string) {
    playerMutex.Lock()
    defer playerMutex.Unlock()
    delete(playerConnections, playerID)
}
// adding conn, to conns map, with proper locking
func RegisterGameConnection(gameID string, conn *websocket.Conn){
	connMutex.Lock()
	defer connMutex.Unlock()
	connections[gameID] = append(connections[gameID], conn)
}

func RegisterGlobalConnection(conn *websocket.Conn) {
	connMutex.Lock()
	defer connMutex.Unlock()
	
	connections["global"] = append(connections["global"], conn)
}

func RemoveGlobalConnection(conn *websocket.Conn) {
	connMutex.Lock()
	defer connMutex.Unlock()
	
	conns := connections["global"]
	for i, c := range conns {
		if c == conn {
			connections["global"] = append(conns[:i], conns[i+1:]...)
			break
		}
	}
}
// removing conn from conns map
func RemoveGameConnection(gameID string, conn *websocket.Conn){
	connMutex.Lock()
	defer func ()  {
		connMutex.Unlock()
		// clean up -- >  can be added to defer function
		if len (connections[gameID]) == 0 {
			delete(connections, gameID)
	}	
	}()

	conns := connections[gameID]
	for i, c := range conns {
		// itereate on conns, when match, renmove the curr, using the slice

		if c == conn {
			connections[gameID] = append(conns[:i], conns[i+1:]...)
			break
		}
	}
	

}

// this function handle the websocket msg, for typegamestate messages, defined earlier
func BroadcastGameState(gameID string, game *games.Game){
	log.Printf("Broadcasting game state for game: %s", gameID)
	connMutex.Lock()
	conns := connections[gameID]
	defer connMutex.Unlock()

	gameJson, err := json.Marshal(game)
	if err != nil{
		log.Printf("Error in marshalling game state : %v", err)
		return 
	}
	message := Message{
		Type: TypeGameState,
		Payload: gameJson,
	}
	messageJson, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	for _, conn := range conns {
		err := conn.WriteMessage(websocket.TextMessage, messageJson)
		if err != nil{
			log.Printf("Error sending message: %v", err)
			conn.Close()
			RemoveGameConnection(gameID, conn)
		}
	}
}

// function to process all the incoming messages for a game

func HandleConnection(gameID string, conn *websocket.Conn){
	log.Printf("Starting HandleConnection for game: %s", gameID)

	log.Printf("Handling connection for game: %s", gameID) 
	defer func ()  {
		conn.Close()
		RemoveGameConnection(gameID, conn)
	}()

	// added read deadline, for 2 mins
	conn.SetReadDeadline(time.Now().Add(time.Minute * 2))
	conn.SetPongHandler(func (string) error {
		conn.SetReadDeadline(time.Now().Add(time.Minute * 2))
		return nil
	})

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	// this ticker will keep the connection alive -- pinging after 30 seconds interval, 
	// 
	go func ()  {
		for range ticker.C {
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}()

	// load the game
	game, err := GetGame(gameID)
	if err != nil {
		log.Printf("Error in loading game : %v", err)
		return
	}
	// sending initial game state
	BroadcastGameState(gameID, game)

	// here we will be processing all the incoming messages from players
	for {
		_, messageData, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Error in reading message %v", err)
			break
		}
		var message Message

		if err := json.Unmarshal(messageData, &message); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		// handle message with message type
		switch message.Type{
		case TypeMove:
			var move games.Move
			if err := json.Unmarshal(message.Payload, &move); err != nil {
				log.Printf("Error unmarshaling move : %v", err)
				continue
			}
			log.Printf("Received move from player %s: %v", move.PlayerID, move)
			//now we have the move, so we make the move
			if err := game.MakeMove(move.PlayerID, move.Column); err != nil{
				
				errMsg := ErrorMessage{Error: err.Error()}
				errJson, _ := json.Marshal(errMsg)
				response := Message{
					Type: TypeError,
					Payload: errJson,
				}
				responseJson, _ := json.Marshal(response)
				conn.WriteMessage(websocket.TextMessage, responseJson)
				continue
			}
			// after making the move, save the game state
			if err := SaveGame(game); err != nil{
				log.Printf("Error in saving the game : %v" , err)
			}

			// broadcast the game status
			BroadcastGameState(gameID, game)

			// TODO: if game is against the bot, make the bot move

			// check if game finished
			if game.Status == games.StatusFinished{
				updatePlayerStats(game)
			}

		case TypeJoinGame:

			var joinRequest struct {
				PlayerID string `json:"playerId"`
			}
			if err := json.Unmarshal(message.Payload, &joinRequest); err != nil {
				log.Printf("Error unmarshaling join request: %v", err)
				continue
			}
			
			// Update the game with the second player
			game.Player2ID = joinRequest.PlayerID
			game.Status = games.StatusActive
			
			log.Printf("Player %s joined game %s", joinRequest.PlayerID, gameID)
			
			// Save the updated game
			if err := SaveGame(game); err != nil {
				log.Printf("Error saving game after join: %v", err)
				
				// Send error response
				errMsg := ErrorMessage{Error: "Failed to save game after join"}
				errJson, _ := json.Marshal(errMsg)
				response := Message{
					Type: TypeError,
					Payload: errJson,
				}
				responseJson, _ := json.Marshal(response)
				conn.WriteMessage(websocket.TextMessage, responseJson)
				continue
			}
			
			// Broadcast the updated game state to all clients
			BroadcastGameState(gameID, game)
		case TypeResetRequest:
            // Handle reset game request
            log.Printf("Received reset game request for game: %s", gameID)
            
            // Parse the reset request
            var resetRequest struct {
                PlayerID string `json:"playerId"`
            }
            if err := json.Unmarshal(message.Payload, &resetRequest); err != nil {
                log.Printf("Error unmarshaling reset request: %v", err)
                continue
            }
            
            // Verify player is in this game
            if resetRequest.PlayerID != game.Player1ID && resetRequest.PlayerID != game.Player2ID {
                log.Printf("Player %s not in game %s", resetRequest.PlayerID, gameID)
                sendErrorMessage(conn, "You are not a player in this game")
                continue
            }
            
            // Reset the game state
            if resetRequest.PlayerID != game.Player1ID && resetRequest.PlayerID != game.Player2ID {
                log.Printf("Player %s not in game %s", resetRequest.PlayerID, gameID)
                sendErrorMessage(conn, "You are not a player in this game")
                continue
            }
            
            // Determine the other player's ID
            otherPlayerID := game.Player1ID
            if resetRequest.PlayerID == game.Player1ID {
                otherPlayerID = game.Player2ID
            }
            
            log.Printf("Player %s requested game reset, waiting for confirmation from %s", 
                      resetRequest.PlayerID, otherPlayerID)
            
            // Broadcast reset request to all connections for this game
            BroadcastResetRequest(gameID, otherPlayerID, resetRequest.PlayerID)
		case TypeResetConfirm:
            // Handle reset confirmation from the other player
            var resetConfirm struct {
                PlayerID string `json:"playerId"`
                Confirm  bool   `json:"confirm"`
            }
            if err := json.Unmarshal(message.Payload, &resetConfirm); err != nil {
                log.Printf("Error unmarshaling reset confirmation: %v", err)
                continue
            }
            
            // Verify player is in this game
            if resetConfirm.PlayerID != game.Player1ID && resetConfirm.PlayerID != game.Player2ID {
                log.Printf("Player %s not in game %s", resetConfirm.PlayerID, gameID)
                sendErrorMessage(conn, "You are not a player in this game")
                continue
            }
            
            if resetConfirm.Confirm {
                // Reset confirmed, reset the game
                game.Status = games.StatusActive
				game.Board = games.NewBoard()
				game.CurrentTurn = games.RedToken
				if ( game.WinnerID == game.Player2ID){
					game.CurrentTurn= games.YellowToken
					}
				game.WinnerID = ""
				game.LastMoveTime = time.Now()
				
				// Save the updated game
				if err := SaveGame(game); err != nil {
					log.Printf("Error saving game after reset: %v", err)
					sendErrorMessage(conn, "Failed to reset game")
					continue
				}
				sendMessageTo := resetConfirm.PlayerID
				if resetConfirm.PlayerID == game.Player1ID {
					sendMessageTo = game.Player2ID
				}
                log.Printf("Game %s has been reset after confirmation from %s", 
                          gameID, sendMessageTo)
                
                // Broadcast the updated game state to all clients
				BroadcastResetGame(gameID)
                BroadcastGameState(gameID, game)
            } else {
                // Reset rejected, notify the other player
                BroadcastResetRejected(gameID, resetConfirm.PlayerID)
            }
            
            
        
		}


	}


}
func BroadcastResetGame(gameID string){
	log.Printf("Broadcasting reset game for game: %s", gameID)
	connMutex.Lock()
	conns := connections[gameID]
	defer connMutex.Unlock()

	// Create reset game message
	message := Message{
		Type: TypeResetGame,
		Payload: nil,
	}
	messageJSON, _ := json.Marshal(message)

	// Send to all connections
	for _, conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, messageJSON); err != nil {
			log.Printf("Error sending reset game: %v", err)
			conn.Close()
			RemoveGameConnection(gameID, conn)
		}
	}
}
func BroadcastResetRequest(gameID string, otherPlayerID string, requestingPlayerID string) {
    log.Printf("Broadcasting reset request for game: %s", gameID)
    connMutex.Lock()
    conn := playerConnections[otherPlayerID]
    defer connMutex.Unlock()

    // Create reset request message
    resetRequestData := struct {
        RequestingPlayerID string `json:"requestingPlayerId"`
    }{
        RequestingPlayerID: requestingPlayerID,
    }
    
    payload, _ := json.Marshal(resetRequestData)
    message := Message{
        Type: "resetRequest",
        Payload: payload,
    }
    
    messageJSON, _ := json.Marshal(message)
    
    // Send to all connections
	if err := conn.WriteMessage(websocket.TextMessage, messageJSON); err != nil {
		log.Printf("Error sending reset request: %v", err)
		conn.Close()
		RemoveGameConnection(gameID, conn)
	}
}

// Function to broadcast reset rejection
func BroadcastResetRejected(gameID string, rejectingPlayerID string) {
    log.Printf("Broadcasting reset rejection for game: %s", gameID)
    connMutex.Lock()
    conns := connections[gameID]
    defer connMutex.Unlock()

    // Create reset rejected message
    resetRejectedData := struct {
        RejectingPlayerID string `json:"rejectingPlayerId"`
    }{
        RejectingPlayerID: rejectingPlayerID,
    }
    
    payload, _ := json.Marshal(resetRejectedData)
    message := Message{
        Type: "resetRejected",
        Payload: payload,
    }
    
    messageJSON, _ := json.Marshal(message)
    
    // Send to all connections
    for _, conn := range conns {
        if err := conn.WriteMessage(websocket.TextMessage, messageJSON); err != nil {
            log.Printf("Error sending reset rejection: %v", err)
            conn.Close()
            RemoveGameConnection(gameID, conn)
        }
    }
}

func updatePlayerStats(game *games.Game ){

	if game.Player1ID == "bot" || game.Player2ID == "bot"{
		return 
	}

	if game.WinnerID != ""{
		player , err := GetPlayer(game.WinnerID)
		if err == nil {
			player.Wins++
			SavePlayer(player)
		}

		loserID := game.Player1ID

		if game.WinnerID == game.Player1ID {
			loserID = game.Player2ID
		}

		loser, err := GetPlayer(loserID)
		if err == nil {
			loser.Losses++
			SavePlayer(loser)
		}
	}
}

func HandleGlobalConnection(conn *websocket.Conn) {
    // Register connection first
    RegisterGlobalConnection(conn)
    
    // Single defer block with all cleanup
    defer func() {
        log.Printf("Closing global connection")
        conn.Close()
        RemoveGlobalConnection(conn)
    }()
    
    // Send a welcome message in the correct Message format
    welcomePayload, _ := json.Marshal(struct {
        Message string `json:"message"`
    }{
        Message: "Successfully connected to game server",
    })
    
    welcomeMsg := Message{
        Type:    "connected",
        Payload: welcomePayload,
    }
    
    welcomeJSON, _ := json.Marshal(welcomeMsg)
    if err := conn.WriteMessage(websocket.TextMessage, welcomeJSON); err != nil {
        log.Printf("Error sending welcome message: %v", err)
        return
    }
    
    // Set read deadline and ping/pong handler
    conn.SetReadDeadline(time.Now().Add(time.Minute * 2))
    conn.SetPongHandler(func (string) error {
        conn.SetReadDeadline(time.Now().Add(time.Minute * 2))
        return nil
    })

    ticker := time.NewTicker(60 * time.Second)
    defer ticker.Stop()
    
    go func() {
        for range ticker.C {
            if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }
        }
    }()
    
    // Process incoming messages
    for {
        _, messageData, err := conn.ReadMessage()
        if err != nil {
            log.Printf("Error reading message: %v", err)
            break
        }
        
        var message Message
        if err := json.Unmarshal(messageData, &message); err != nil {
            log.Printf("Error unmarshaling message: %v", err)
            continue
        }
        
        switch message.Type {
        case TypeJoinGame:
            log.Printf("Received join request")
            var joinRequest struct {
                PlayerID string `json:"playerId"`
            }
            if err := json.Unmarshal(message.Payload, &joinRequest); err != nil {
                log.Printf("Error unmarshaling join request: %v", err)
                continue
            }
            RegisterPlayerConnection(joinRequest.PlayerID, conn)
            // Try to find a waiting game
            waitingGame, err := FindWaitingGame()
            
            if err == nil && waitingGame != nil {
                // Found a waiting game, join it
                log.Printf("Joining waiting game %s for playerId: %s", waitingGame.ID, joinRequest.PlayerID)

                waitingGame.Player2ID = joinRequest.PlayerID
                waitingGame.Status = games.StatusActive
                
                if err := SaveGame(waitingGame); err != nil {
                    log.Printf("Error saving game after join: %v", err)
                    sendErrorMessage(conn, "Failed to save game after join")
                    continue
                }
                
                // Now both players are known, reply with gameStart to THIS connection
                sendGameStartMessage(conn, waitingGame)
                player1Conn := GetPlayerConnection(waitingGame.Player1ID)
                if player1Conn != nil {
                    sendGameStartMessage(player1Conn, waitingGame)
                } else {
                    log.Printf("Warning: Could not find connection for player1: %s", waitingGame.Player1ID)
                }
                
                
                // Don't transition this connection - client will create a new one
                
            } else {
                log.Printf("Creating new game for %s", joinRequest.PlayerID)
                
                // No waiting game found, create a new one
                newGame := games.NewGame(games.OnlineMultiplayer, joinRequest.PlayerID, "")
                
                if err := SaveGame(newGame); err != nil {
                    log.Printf("Error creating new game: %v", err)
                    sendErrorMessage(conn, "Failed to create new game")
                    continue
                }
                
                // Send gameCreated message back to this connection
                sendGameCreatedMessage(conn, newGame)
                
                // Don't transition this connection - client will create a new one
            }
        }
    }
}

// Send a gameCreated message to a connection (for when a new game is created with only player1)
func sendGameCreatedMessage(conn *websocket.Conn, game *games.Game) {
    gameCreatedData := struct {
        GameID    string `json:"gameId"`
        Player1ID string `json:"player1Id"`
    }{
        GameID:    game.ID,
        Player1ID: game.Player1ID,
    }
    
    payload, _ := json.Marshal(gameCreatedData)
    message := Message{
        Type:    "gameCreated",
        Payload: payload,
    }
    
    messageJSON, _ := json.Marshal(message)
    if err := conn.WriteMessage(websocket.TextMessage, messageJSON); err != nil {
        log.Printf("Error sending game created message: %v", err)
    }
}
// Function to send a game start message to a specific connection
func sendGameStartMessage(conn *websocket.Conn, game *games.Game) {
    // Create the game start data structure
    gameStartData := struct {
        GameID    string `json:"gameId"`
        Player1ID string `json:"player1Id"`
        Player2ID string `json:"player2Id"`
    }{
        GameID:    game.ID,
        Player1ID: game.Player1ID,
        Player2ID: game.Player2ID,
    }
    
    // Marshal the game start data to JSON
    payload, err := json.Marshal(gameStartData)
    if err != nil {
        log.Printf("Error marshaling game start data: %v", err)
        return
    }
    
    // Create the message with type and payload
    message := Message{
        Type:    "gameStart",
        Payload: payload,
    }
    
    // Marshal the entire message to JSON
    messageJSON, err := json.Marshal(message)
    if err != nil {
        log.Printf("Error marshaling game start message: %v", err)
        return
    }
    
    // Send the message to the connection
    if err := conn.WriteMessage(websocket.TextMessage, messageJSON); err != nil {
        log.Printf("Error sending game start message: %v", err)
    } else {
        log.Printf("Sent gameStart message to client for game %s", game.ID)
    }
}

// Helper function to send error messages
func sendErrorMessage(conn *websocket.Conn, errorText string) {
    errMsg := ErrorMessage{Error: errorText}
    errJSON, _ := json.Marshal(errMsg)
    response := Message{
        Type:    TypeError,
        Payload: errJSON,
    }
    responseJSON, _ := json.Marshal(response)
    conn.WriteMessage(websocket.TextMessage, responseJSON)
}

// Add this function to find a waiting game
func FindWaitingGame() (*games.Game, error) {
	gameMutex.RLock()
	defer gameMutex.RUnlock()
	
	for _, game := range gamesMap {
		if game.Status == games.StatusWaiting {
			return game, nil
		}
	}
	
	return nil, errors.New("no waiting game found")
}