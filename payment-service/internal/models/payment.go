package models

import "time"

type Payment struct {
	ID              string
	OrderID         string
	UserID          string
	Amount          int64
	MetodoPagoToken string
	Status          string
	CreatedAt       time.Time
}
