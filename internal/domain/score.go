package domain

import "time"

type Score struct {
	BoardID  int64
	UserID   string
	Score    int64
	ScoredAt time.Time
}
