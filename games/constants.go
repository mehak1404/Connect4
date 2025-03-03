package games

const (
	BoardWidth  = 7
	BoardHeight = 6
	
	// Player tokens
	EmptyCell = 0
	RedToken  = 1
	YellowToken = 2

	StatusWaiting  GameStatus = "waiting"
	StatusActive   GameStatus = "active"
	StatusFinished GameStatus = "finished"

	SinglePlayer GameType = "single"
	LocalMultiplayer GameType = "local"
	OnlineMultiplayer GameType = "online"


)