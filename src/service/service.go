package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"movieservice/client"
)

// ограничимся разумным максимумом, чтобы не убить сервис большим диапазоном
const maxBatch = 1000

var (
	ErrInvalidRange  = errors.New("from must be >= 1 and to must be >= from")
	ErrRangeTooLarge = fmt.Errorf("range is too large (max %d movies)", maxBatch)
)

// describeFetchError возвращает безопасное для клиента описание ошибки загрузки
// (без URL и других внутренних деталей).
func describeFetchError(err error) string {
	var statusErr *client.StatusError
	switch {
	case errors.Is(err, client.ErrNotFound):
		return "not found"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, client.ErrBadResponse):
		return "invalid response from stand"
	case errors.As(err, &statusErr):
		return statusErr.Error()
	default:
		return "stand is unavailable"
	}
}

type LoadError struct {
	ID    int    `json:"id"`
	Error string `json:"error"`
}

// Item - элемент ответа /load-movies: либо фильм, либо ошибка загрузки по этому id.
type Item struct {
	Movie *client.Movie
	Err   *LoadError
}

func (i Item) MarshalJSON() ([]byte, error) {
	if i.Err != nil {
		return json.Marshal(i.Err)
	}
	return json.Marshal(i.Movie)
}

type LoadResult struct {
	Movies []*client.Movie `json:"movies"`
	Errors []LoadError     `json:"errors,omitempty"`
	// Items - те же данные в порядке id (фильмы и ошибки вперемешку), для HTTP-ответа
	Items []Item `json:"-"`
}

type Service struct{ c *client.Client }

func New(c *client.Client) *Service { return &Service{c: c} }

func (s *Service) LoadMovies(ctx context.Context, from, to int) (*LoadResult, error) {
	if from < 1 || to < from {
		return nil, ErrInvalidRange
	}
	if to-from+1 > maxBatch {
		return nil, ErrRangeTooLarge
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	n := to - from + 1
	movies := make([]*client.Movie, n)
	errs := make([]error, n)
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			m, err := s.c.Fetch(ctx, from+i)
			if err != nil {
				errs[i] = err
				return
			}
			movies[i] = m
		}(i)
	}
	wg.Wait()

	res := &LoadResult{Items: make([]Item, 0, n)}
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			le := LoadError{ID: from + i, Error: describeFetchError(errs[i])}
			res.Errors = append(res.Errors, le)
			res.Items = append(res.Items, Item{Err: &le})
		} else {
			res.Movies = append(res.Movies, movies[i])
			res.Items = append(res.Items, Item{Movie: movies[i]})
		}
	}
	return res, nil
}

func (s *Service) GetLastMovie(ctx context.Context) (*client.Movie, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	first, err := s.c.Fetch(ctx, 1)
	if errors.Is(err, client.ErrNotFound) {
		return nil, client.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	last := first
	lo, hi := 1, 2
	for {
		if hi > 1_000_000 {
			break
		}
		m, err := s.c.Fetch(ctx, hi)
		if errors.Is(err, client.ErrNotFound) {
			break
		}
		if err != nil {
			return nil, err
		}
		last = m
		lo, hi = hi, hi*2
	}
	for lo < hi-1 {
		mid := (lo + hi) / 2
		m, err := s.c.Fetch(ctx, mid)
		if errors.Is(err, client.ErrNotFound) {
			hi = mid
			continue
		}
		if err != nil {
			return nil, err
		}
		lo, last = mid, m
	}
	return last, nil
}
