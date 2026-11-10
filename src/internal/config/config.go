package config

import "os"

type Config struct {
	MovieBaseURL string
	HTTPAddr     string
	GRPCAddr     string
}

func Load() Config {
	return Config{
		MovieBaseURL: envOr("MOVIE_BASE_URL", "https://homeworksite.site"),
		HTTPAddr:     envOr("HTTP_ADDR", ":8080"),
		GRPCAddr:     envOr("GRPC_ADDR", ":9090"),
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
