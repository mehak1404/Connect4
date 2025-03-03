package db

import (
	"connect4/games"
	"errors"
	"sync"
	"time"
)

var (
	gamesMap = make(map[string]*games.Game)
	players = make(map[string]*games.Player)

	gameMutex = &sync.RWMutex{}
	playerMutex = &sync.RWMutex{}
)

func Initialize() error {
	return nil
}


// -------------------------- GAME ---------------------------

func SaveGame(g *games.Game) error {
	gameMutex.Lock()
	defer gameMutex.Unlock()

	gamesMap[g.ID] = g
	return nil
}

func GetGame(gameID string) (*games.Game, error){
	gameMutex.RLock()
	defer gameMutex.RUnlock()
	
	game, exists := gamesMap[gameID]
	if !exists {
		return nil, errors.New("game not found")
	}
	
	return game, nil
}

func CreateGame(g * games.Game) error {
	gameMutex.Lock()
	defer gameMutex.Unlock()

	gamesMap[g.ID] = g
	return nil
}

func ListGame() ([] * games.Game, error){
	gameMutex.RLock()
	defer gameMutex.RUnlock()

	result := make([] * games.Game, 0, len(gamesMap))
	for _, g :=  range gamesMap {
		result = append(result, g)
	}
	return result, nil
}

// ----------------- PLAYER -----------------------

func SavePlayer(p * games.Player) error {
	playerMutex.Lock()
	defer playerMutex.Unlock()

	players[p.ID] = p
	return nil
}

func GetPlayer(playerID string) (*games.Player, error){
	playerMutex.RLock()
	defer playerMutex.RUnlock()

	player, exists := players[playerID]
	if ! exists {
		return nil, errors.New("player not found")
	}
	return player, nil
}

func CreatePlayer(p *games.Player) error {
	playerMutex.Lock()
	defer playerMutex.Unlock()
	
	// Check if username already exists
	for _, existingPlayer := range players {
		if existingPlayer.Username == p.Username {
			return errors.New("username already taken")
		}
	}
	
	// Generate ID if not provided
	if p.ID == "" {
		p.ID = "player_" + time.Now().Format("20060102150405")
	}
	
	// Set creation time if not set
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	
	players[p.ID] = p
	return nil
}

// ListPlayers returns all players in the database
func ListPlayers() ([]*games.Player, error) {
	playerMutex.RLock()
	defer playerMutex.RUnlock()
	
	result := make([]*games.Player, 0, len(players))
	for _, p := range players {
		result = append(result, p)
	}
	
	return result, nil
}

// GetLeaderboard returns players sorted by win count
func GetLeaderboard(limit int) ([]*games.Player, error) {
	players, err := ListPlayers()
	if err != nil {
		return nil, err
	}
	
	// Sort players by wins (descending)
	// In a real database, this would be done with a query
	for i := 0; i < len(players); i++ {
		for j := i + 1; j < len(players); j++ {
			if players[j].Wins > players[i].Wins {
				players[i], players[j] = players[j], players[i]
			}
		}
	}
	
	// Apply limit if specified
	if limit > 0 && limit < len(players) {
		players = players[:limit]
	}
	
	return players, nil
}