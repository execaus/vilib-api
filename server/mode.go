package server

import "errors"

type Mode string

const (
	HybridMode      Mode = "hybrid"
	ProductionMode       = "production"
	DevelopmentMode      = "development"
)

var (
	ErrInvalidServerMode = errors.New("invalid server mode")
)
