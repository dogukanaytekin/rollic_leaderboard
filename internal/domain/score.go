package domain

import "time"

type Score struct {
	BoardID  int64
	UserID   string
	Score    int64
	ScoredAt time.Time
}

type TopScoreEntry struct {
	UserID string `json:"userId"`
	Score  int64  `json:"score"`
}

type Surroundings struct {
	User  TopScoreEntry   `json:"user"`
	Above []TopScoreEntry `json:"above"`
	Below []TopScoreEntry `json:"below"`
}
