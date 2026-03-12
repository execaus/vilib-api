package models

import "time"

type User struct {
	ID        string
	Name      string
	Surname   string
	Email     string
	CreatedAt time.Time
}
