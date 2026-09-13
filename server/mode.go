package server

import "errors"

type Mode string

const (
	HybridMode      Mode = "hybrid"
	ProductionMode  Mode = "production"
	DevelopmentMode Mode = "development"
)

var (
	ErrInvalidServerMode = errors.New("invalid server mode")
)
