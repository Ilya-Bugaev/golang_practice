package main

import (
	"log"
	"net"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"movieservice/client"
	"movieservice/config"
	"movieservice/grpcapi"
	"movieservice/httpapi"
	"movieservice/proto"
	"movieservice/service"
)

func main() {
	cfg := config.Load()
	svc := service.New(client.New(cfg.MovieBaseURL))

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.New(svc).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		// больше, чем таймаут service.LoadMovies (60 с), иначе ответ оборвётся
		WriteTimeout: 90 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		log.Printf("HTTP on %s → http://localhost%s/swagger/index.html", cfg.HTTPAddr, cfg.HTTPAddr)
		log.Fatal(httpServer.ListenAndServe())
	}()

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Fatalf("grpc listen: %v", err)
	}
	gs := grpc.NewServer()
	proto.RegisterMovieServiceServer(gs, grpcapi.New(svc))
	reflection.Register(gs)
	log.Printf("gRPC on %s", cfg.GRPCAddr)
	log.Fatal(gs.Serve(lis))
}
