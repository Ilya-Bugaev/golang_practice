package usecase

import (
	"context"
	"errors"
	"sync"
	"time"

	"movieservice/internal/domain"
)

// describeFetchError возвращает безопасное для клиента описание ошибки загрузки
// (без URL и других внутренних деталей).
func describeFetchError(err error) string {
	var statusErr *domain.StatusError
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return "not found"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, domain.ErrBadResponse):
		return "invalid response from stand"
	case errors.As(err, &statusErr):
		return statusErr.Error()
	default:
		return "stand is unavailable"
	}
}

type movieService struct {
	provider MovieProvider
}

// NewMovieService создает новый экземпляр сервиса фильмов
func NewMovieService(provider MovieProvider) MovieUseCase {
	return &movieService{
		provider: provider,
	}
}

func (s *movieService) LoadMovies(ctx context.Context, from, to int) (*LoadResult, error) {
	if from < 1 || to < from {
		return nil, domain.ErrInvalidRange
	}
	if to-from+1 > domain.MaxBatch {
		return nil, domain.ErrRangeTooLarge
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	n := to - from + 1
	movies := make([]*domain.Movie, n)
	errs := make([]error, n)
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			m, err := s.provider.GetMovie(ctx, from+i)
			if err != nil {
				errs[i] = err
				return
			}
			movies[i] = &m
		}(i)
	}
	wg.Wait()

	res := &LoadResult{Entries: make([]Entry, 0, n)}
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			le := domain.LoadError{ID: from + i, Error: describeFetchError(errs[i])}
			res.Errors = append(res.Errors, le)
			res.Entries = append(res.Entries, Entry{Err: &le})
		} else {
			res.Movies = append(res.Movies, movies[i])
			res.Entries = append(res.Entries, Entry{Movie: movies[i]})
		}
	}
	return res, nil
}

func (s *movieService) GetLastMovie(ctx context.Context) (domain.Movie, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	first, err := s.provider.GetMovie(ctx, 1)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Movie{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Movie{}, err
	}
	last := first
	lo, hi := 1, 2
	for {
		if hi > 1_000_000 {
			break
		}
		m, err := s.provider.GetMovie(ctx, hi)
		if errors.Is(err, domain.ErrNotFound) {
			break
		}
		if err != nil {
			return domain.Movie{}, err
		}
		last = m
		lo, hi = hi, hi*2
	}
	for lo < hi-1 {
		mid := (lo + hi) / 2
		m, err := s.provider.GetMovie(ctx, mid)
		if errors.Is(err, domain.ErrNotFound) {
			hi = mid
			continue
		}
		if err != nil {
			return domain.Movie{}, err
		}
		lo, last = mid, m
	}
	return last, nil
}
