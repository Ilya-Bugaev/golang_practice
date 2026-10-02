package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Movie struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Year     int    `json:"year"`
	Director string `json:"director"`
}

// Result хранит результат обработки одного фильма
type Result struct {
	ID      int
	Success bool
	Message string
}

// classifyRequestError определяет тип сетевой ошибки (таймаут, отмена, соединение)
// и возвращает сообщение без URL
func classifyRequestError(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return errors.New("таймаут")
	case errors.Is(err, context.Canceled):
		return errors.New("запрос отменён")
	}
	if inner := errors.Unwrap(err); inner != nil {
		err = inner
	}
	return fmt.Errorf("ошибка соединения: %w", err)
}

// fetchMovie выполняет HTTP-запрос и парсит ответ
func fetchMovie(ctx context.Context, client *http.Client, id int, timeoutSec float64) (string, error) {
	url := fmt.Sprintf("https://homeworksite.site/%d/info.0.json", id)

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec*float64(time.Second)))
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("создание запроса: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", classifyRequestError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var movie Movie
	if err := json.NewDecoder(resp.Body).Decode(&movie); err != nil {
		if reqCtx.Err() != nil {
			return "", classifyRequestError(reqCtx.Err())
		}
		return "", fmt.Errorf("парсинг JSON: %w", err)
	}

	return fmt.Sprintf("%d — %s — %d — %s", movie.ID, movie.Title, movie.Year, movie.Director), nil
}

// sendJobs отправляет задачи в channel до отмены контекста
func sendJobs(ctx context.Context, from, to int, jobs chan<- int) {
	for id := from; id <= to; id++ {
		select {
		case <-ctx.Done():
			return
		case jobs <- id:
		}
	}
}

// collectResults собирает и выводит результаты обработки
func collectResults(ctx context.Context, totalFilms int, results <-chan Result) {
	collected := 0
	for collected < totalFilms {
		select {
		case <-ctx.Done():
			fmt.Fprintln(os.Stderr, "Завершение работы из-за отмены контекста.")
			return
		case res, ok := <-results:
			if !ok {
				return
			}
			if res.Success {
				fmt.Println(res.Message)
			} else {
				fmt.Fprintln(os.Stderr, res.Message)
			}
			collected++
		}
	}
}

func main() {
	var from, to, workers int
	var timeout float64

	flag.IntVar(&from, "from", 0, "ID первого фильма (обязательный, >= 1)")
	flag.IntVar(&to, "to", 0, "ID последнего фильма (обязательный, >= from)")
	flag.IntVar(&workers, "workers", 10, "Количество воркеров в пуле")
	flag.Float64Var(&timeout, "timeout", 5.0, "Таймаут одного HTTP-запроса в секундах")

	flag.Parse()

	provided := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { provided[f.Name] = true })
	if !provided["from"] || !provided["to"] {
		fmt.Fprintln(os.Stderr, "Ошибка: флаги --from и --to являются обязательными")
		os.Exit(1)
	}
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "Ошибка: неожиданные аргументы: %v\n", flag.Args())
		os.Exit(1)
	}
	if from < 1 || to < 1 {
		fmt.Fprintln(os.Stderr, "Ошибка: значения --from и --to должны быть >= 1")
		os.Exit(1)
	}
	if from > to {
		fmt.Fprintln(os.Stderr, "Ошибка: значение --from не может быть больше --to")
		os.Exit(1)
	}
	if workers <= 0 {
		fmt.Fprintln(os.Stderr, "Ошибка: количество воркеров (--workers) должно быть > 0")
		os.Exit(1)
	}
	if !(timeout > 0) || timeout > 3600 { // !(>0) также отсекает NaN
		fmt.Fprintln(os.Stderr, "Ошибка: таймаут (--timeout) должен быть > 0 и <= 3600 секунд")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nПолучен сигнал прерывания, отмена оставшихся запросов...")
		cancel()
	}()

	totalFilms := to - from + 1
	jobs := make(chan int, workers)
	results := make(chan Result, workers)

	client := &http.Client{
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			MaxIdleConnsPerHost: workers,
		},
	}

	var wg sync.WaitGroup

	for w := 1; w <= workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for id := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				res := Result{ID: id}
				msg, err := fetchMovie(ctx, client, id, timeout)
				if err != nil {
					res.Success = false
					res.Message = fmt.Sprintf("ID %d: ошибка - %v", id, err)
				} else {
					res.Success = true
					res.Message = msg
				}
				select {
				case results <- res:
				case <-ctx.Done():
					return
				}
			}
		}(w)
	}

	go func() {
		sendJobs(ctx, from, to, jobs)
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	collectResults(ctx, totalFilms, results)

	if ctx.Err() != nil {
		wg.Wait()
		os.Exit(130)
	}
}
