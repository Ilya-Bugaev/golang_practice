package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	ErrNotFound    = errors.New("not found")
	ErrBadResponse = errors.New("invalid response from stand")
)

// StatusError - стенд ответил неожиданным HTTP-статусом (не 200 и не 404).
type StatusError struct{ Code int }

func (e *StatusError) Error() string { return fmt.Sprintf("stand returned %d", e.Code) }

// Movie - описание фильма в формате стенда:
// {"id":1,"title":"To Rome with Love","year":2012,"director":"Woody Allen"}
type Movie struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Year     int    `json:"year"`
	Director string `json:"director"`
}

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Fetch(ctx context.Context, id int) (*Movie, error) {
	url := fmt.Sprintf("%s/%d/info.0.json", c.baseURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stand error: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, ErrNotFound
	default:
		return nil, &StatusError{Code: resp.StatusCode}
	}
	var m Movie
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadResponse, err)
	}
	return &m, nil
}
