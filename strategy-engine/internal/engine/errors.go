package engine

import "errors"

var (
	ErrStrategyAlreadyExists = errors.New("strategy already exists")
	ErrStrategyNotFound      = errors.New("strategy not found")
)
