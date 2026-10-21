package domain

import "errors"

var (
	ErrNotFound             = errors.New("not found")
	ErrAlreadyExists        = errors.New("already exists")
	ErrInvalidPath          = errors.New("invalid path")
	ErrInvalidArgument      = errors.New("invalid argument")
	ErrUnsupportedMediaType = errors.New("unsupported media type")
	ErrResourceTooLarge     = errors.New("resource too large")
	ErrAutomationBusy       = errors.New("automation is already running")
)
