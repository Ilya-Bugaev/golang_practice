package usecase

import (
	"context"
	"movieservice/internal/domain"
)

// MovieProvider интерфейс для получения фильма из внешнего источника
type MovieProvider interface {
	GetMovie(ctx context.Context, id int) (domain.Movie, error)
}

// MovieUseCase интерфейс бизнес-логики работы с фильмами
type MovieUseCase interface {
	LoadMovies(ctx context.Context, from, to int) (*LoadResult, error)
	GetLastMovie(ctx context.Context) (domain.Movie, error)
}
