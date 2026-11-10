package domain

import (
	"errors"
	"fmt"
)

// MaxBatch - разумный максимум фильмов в одном диапазоне загрузки,
// чтобы не убить сервис слишком большим запросом.
const MaxBatch = 1000

// Основные доменные ошибки
var (
	ErrNotFound    = errors.New("not found")
	ErrBadResponse = errors.New("invalid response from stand")

	// Ошибки валидации входных параметров
	ErrInvalidRange  = errors.New("from must be >= 1 and to must be >= from")
	ErrRangeTooLarge = fmt.Errorf("range is too large (max %d movies)", MaxBatch)
)

// StatusError - стенд ответил неожиданным HTTP-статусом (не 200 и не 404).
type StatusError struct {
	Code int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("stand returned %d", e.Code)
}

// LoadError - ошибка загрузки конкретного фильма
type LoadError struct {
	ID    int    `json:"id"`
	Error string `json:"error"`
}
