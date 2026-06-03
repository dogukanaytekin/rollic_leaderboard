package domain

import "time"

type Schedule struct {
	Type            string `json:"type"`
	IntervalSeconds int64  `json:"intervalSeconds"`
}

type Board struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Schedule    *Schedule `json:"schedule,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}
