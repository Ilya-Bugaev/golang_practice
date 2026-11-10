package adapter

import (
	"context"
	"encoding/json"

	"fmt"
	"movieservice/internal/domain"
	"net/http"
	"strings"
	"time"
)

// MovieClient клиент для взаимодействия с фильм-стендои
type MovieClient struct {
	baseURL string
	http    *http.Client
}

// NewMovieClient создает новый клиент для работы с фильм-стендои
func NewMovieClient(baseURL string) *MovieClient {
	return &MovieClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// GetMovie получает фильм по ID из внешнего стенда
func (c *MovieClient) GetMovie(ctx context.Context, id int) (domain.Movie, error) {
	url := fmt.Sprintf("%s/%d/info.0.json", c.baseURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return domain.Movie{}, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return domain.Movie{}, fmt.Errorf("stand error: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return domain.Movie{}, domain.ErrNotFound
	default:
		return domain.Movie{}, &domain.StatusError{Code: resp.StatusCode}
	}

	var m domain.Movie
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return domain.Movie{}, fmt.Errorf("%w: %v", domain.ErrBadResponse, err)
	}
	return m, nil
}
