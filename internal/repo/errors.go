package repo

import "errors"

// Общие ошибки репозиториев.
var (
	// ErrNotFound — запись не найдена в БД.
	ErrNotFound = errors.New("not found")

	// ErrAlreadyExists — запись уже существует (конфликт уникальности).
	ErrAlreadyExists = errors.New("already exists")

	// ErrInvalidState — операция невозможна в текущем состоянии.
	ErrInvalidState = errors.New("invalid state")
)
