package db

import (
	"connect4/games"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type MessageType string

// message type for websocket meesages
const (

	TypeGameState MessageType = "gameState"
	TypeMove MessageType = "move"
	TypeError MessageType = "error"
	TypeJoinGame MessageType = "joinGame"

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

// adding conn, to conns map, with proper locking
func RegisterGameConnection(gameID string, conn *websocket.Conn){
	connMutex.Lock()
	defer connMutex.Unlock()
	connections[gameID] = append(connections[gameID], conn)
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

