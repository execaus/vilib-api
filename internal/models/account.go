package models

import "time"

type Account struct {
	ID        string
	Name      string
	Owner     string
	Email     string
	CreatedAt time.Time
}
