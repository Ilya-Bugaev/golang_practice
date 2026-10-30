package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"movieservice/client"
	"movieservice/service"
)

type Handler struct{ svc *service.Service }

func New(svc *service.Service) *Handler { return &Handler{svc: svc} }

// Handler возвращает корневой http.Handler: маршруты, обёрнутые в logging-middleware.
func (h *Handler) Handler() http.Handler {
	mux := http.NewServeMux()

	// маршруты с методом (Go 1.22+): остальные методы попадают в methodNotAllowed
	mux.HandleFunc("GET /load-movies", h.loadMovies)
	mux.HandleFunc("GET /last-movie", h.lastMovie)
	mux.HandleFunc("GET /swagger/index.html", serveSwaggerHTML)
	mux.HandleFunc("GET /swagger/doc.json", serveSwaggerJSON)

	mux.HandleFunc("/load-movies", methodNotAllowed(http.MethodGet))
	mux.HandleFunc("/last-movie", methodNotAllowed(http.MethodGet))
	mux.HandleFunc("/swagger/index.html", methodNotAllowed(http.MethodGet))
	mux.HandleFunc("/swagger/doc.json", methodNotAllowed(http.MethodGet))
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
	})

	return logging(mux)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// logging пишет метод, путь, статус и длительность каждого запроса.
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.RequestURI(), rec.status, time.Since(start))
	})
}


type errorBody struct {
	Error errorInfo `json:"error"`
}

type errorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: errorInfo{Code: code, Message: message}})
}

func methodNotAllowed(allow ...string) http.HandlerFunc {
	allowed := ""
	for i, m := range allow {
		if i > 0 {
			allowed += ", "
		}
		allowed += m
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", allowed)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed",
			fmt.Sprintf("method %s is not allowed, use %s", r.Method, allowed))
	}
}

// writeStandError превращает ошибку обращения к стенду в понятный HTTP-ответ.
// Внутренние детали клиенту не отдаём, только пишем в лог.
func writeStandError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, client.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "no movies found on the stand")
	case errors.Is(err, context.DeadlineExceeded):
		writeError(w, http.StatusGatewayTimeout, "stand_timeout", "the movie stand did not respond in time")
	case errors.Is(err, context.Canceled):
		writeError(w, http.StatusServiceUnavailable, "request_canceled", "the request was canceled")
	case errors.Is(err, client.ErrBadResponse):
		log.Printf("stand error: %v", err)
		writeError(w, http.StatusBadGateway, "stand_error", "the movie stand returned an invalid response")
	default:
		log.Printf("stand error: %v", err)
		writeError(w, http.StatusBadGateway, "stand_error", "the movie stand is unavailable")
	}
}

func intParam(q url.Values, name string) (int, error) {
	raw := q.Get(name)
	if raw == "" {
		return 0, fmt.Errorf("query parameter %q is required", name)
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("query parameter %q must be an integer", name)
	}
	return v, nil
}

func (h *Handler) loadMovies(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from, err := intParam(q, "from")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_argument", err.Error())
		return
	}
	to, err := intParam(q, "to")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_argument", err.Error())
		return
	}

	res, err := h.svc.LoadMovies(r.Context(), from, to)
	switch {
	case errors.Is(err, service.ErrInvalidRange), errors.Is(err, service.ErrRangeTooLarge):
		writeError(w, http.StatusBadRequest, "invalid_argument", err.Error())
		return
	case err != nil:
		log.Printf("load-movies error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	// JSON-массив в порядке id: фильм либо {"id":N,"error":"..."} для неудавшихся
	writeJSON(w, http.StatusOK, res.Items)
}

func (h *Handler) lastMovie(w http.ResponseWriter, r *http.Request) {
	m, err := h.svc.GetLastMovie(r.Context())
	if err != nil {
		writeStandError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

const swaggerHTML = `<!DOCTYPE html>
<html><head><title>Movies API</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist/swagger-ui.css">
</head><body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist/swagger-ui-bundle.js"></script>
<script>SwaggerUIBundle({url:'/swagger/doc.json',dom_id:'#swagger-ui'})</script>
</body></html>`

const swaggerJSON = `{
  "swagger":"2.0","info":{"title":"Movies API","version":"1.0"},"basePath":"/",
  "produces":["application/json"],
  "paths":{
    "/load-movies":{"get":{"summary":"Load movies in range [from; to]","parameters":[
      {"name":"from","in":"query","required":true,"type":"integer","minimum":1},
      {"name":"to","in":"query","required":true,"type":"integer","minimum":1}],
      "responses":{
        "200":{"description":"JSON array in id order. Each element is a movie, or {\"id\":N,\"error\":\"...\"} if that id failed to load","schema":{"type":"array","items":{"type":"object"}}},
        "400":{"description":"Invalid from/to","schema":{"$ref":"#/definitions/Error"}},
        "405":{"description":"Method not allowed","schema":{"$ref":"#/definitions/Error"}}}}},
    "/last-movie":{"get":{"summary":"Get last movie","responses":{
      "200":{"description":"OK","schema":{"type":"object"}},
      "404":{"description":"No movies on the stand","schema":{"$ref":"#/definitions/Error"}},
      "405":{"description":"Method not allowed","schema":{"$ref":"#/definitions/Error"}},
      "502":{"description":"Stand error","schema":{"$ref":"#/definitions/Error"}},
      "504":{"description":"Stand timeout","schema":{"$ref":"#/definitions/Error"}}}}}
  },
  "definitions":{
    "Error":{"type":"object","properties":{"error":{"type":"object","properties":{
      "code":{"type":"string","example":"invalid_argument"},
      "message":{"type":"string","example":"query parameter \"from\" is required"}}}}}
  }
}`

func serveSwaggerHTML(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(swaggerHTML))
}

func serveSwaggerJSON(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(swaggerJSON))
}
