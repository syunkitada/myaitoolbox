package domain

import "errors"

var (
	ErrNotFound        = errors.New("not found")
	ErrAlreadyExists   = errors.New("already exists")
	ErrInvalidPath     = errors.New("invalid path")
	ErrInvalidArgument = errors.New("invalid argument")
)
