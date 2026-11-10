package usecase

import (
	"movieservice/internal/domain"
)

// Entry - результат загрузки одного фильма: либо сам фильм, либо ошибка загрузки по этому id.
// Ровно одно из полей Movie/Err заполнено.
type Entry struct {
	Movie *domain.Movie
	Err   *domain.LoadError
}

// LoadResult - результат загрузки диапазона фильмов.
// Транспортно-независимая структура: сериализацией занимается handler.
type LoadResult struct {
	// Movies - успешно загруженные фильмы (в порядке id).
	Movies []*domain.Movie
	// Errors - ошибки загрузки отдельных фильмов (в порядке id).
	Errors []domain.LoadError
	// Entries - фильмы и ошибки вперемешку, в порядке id.
	Entries []Entry
}
