// main.go - Entry point for our Go server
package main

import (
	"log"
	"net/http"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"connect4/api"
	"connect4/db"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections in development
	},
}

func main() {
	// Initialize database connection
	if err := db.Initialize(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	
	// Create router
	router := mux.NewRouter()
	
	// REST API endpoints
	router.HandleFunc("/api/players", api.GetPlayers).Methods("GET")
	router.HandleFunc("/api/players", api.CreatePlayer).Methods("POST")
	router.HandleFunc("/api/players/{id}", api.GetPlayer).Methods("GET")
	router.HandleFunc("/api/leaderboard", api.GetLeaderboard).Methods("GET")
	
	router.HandleFunc("/api/games", api.CreateGame).Methods("POST")
	router.HandleFunc("/api/games", api.GetGames).Methods("GET")
	router.HandleFunc("/api/games/{id}", api.GetGame).Methods("GET")
	router.HandleFunc("/api/games/{id}", api.GetGame).Methods("Put")
	router.HandleFunc("/api/games/{id}/move", api.MakeMove).Methods("POST")
	router.HandleFunc("/api/games/{id}/reset", api.ResetGame).Methods("POST")
	router.HandleFunc("/api/matchmaking", api.MatchMaking).Methods("POST")

	// WebSocket endpoint for real-time gameplay
	router.HandleFunc("/ws/game/{id}", handleGameWebSocket)
	router.HandleFunc("/ws/", handleGlobalConnection);
	
	// Start the server
	log.Println("Starting server on :9000")
	log.Fatal(http.ListenAndServe(":9000", router))
}

func handleGameWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received WebSocket connection attempt from: %s", r.RemoteAddr)
    
	vars := mux.Vars(r)
	gameID := vars["id"]
	
	
	conn, err := upgrader.Upgrade(w, r, nil)
	// conn.CheckOrigin = func(r *http.Request) bool {
    //     return true // Accept all connections for testing
    // }
	if err != nil {
		log.Println("Failed to upgrade connection:", err)
		return
	}
	
	log.Printf("WebSocket connection established with: %s", r.RemoteAddr)
    
	// Register this connection with our game manager
	api.RegisterGameConnection(gameID, conn)
	
	// Handle incoming WebSocket messages
	go db.HandleConnection(gameID, conn)
}
func handleGlobalConnection(w http.ResponseWriter, r *http.Request) {
    log.Printf("Received WebSocket connection attempt from: %s", r.RemoteAddr)
    
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Println("Failed to upgrade connection:", err)
        return
    }
    
    // REMOVE THE defer conn.Close() HERE
    
    log.Printf("WebSocket connection established with: %s", r.RemoteAddr)
    
    // Let the db.HandleGlobalConnection function manage the connection lifecycle
    db.HandleGlobalConnection(conn)
}