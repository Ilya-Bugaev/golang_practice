package grpcapi

import (
	"context"
	"errors"
	"log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"movieservice/client"
	pb "movieservice/proto"
	"movieservice/service"
)

type Server struct {
	pb.UnimplementedMovieServiceServer
	svc *service.Service
}

func New(svc *service.Service) *Server { return &Server{svc: svc} }

func (s *Server) LoadMovies(ctx context.Context, req *pb.LoadMoviesRequest) (*pb.LoadMoviesResponse, error) {
	res, err := s.svc.LoadMovies(ctx, int(req.From), int(req.To))
	if err != nil {
		// валидация диапазона выполняется в сервисе (единое место для HTTP и gRPC)
		if errors.Is(err, service.ErrInvalidRange) || errors.Is(err, service.ErrRangeTooLarge) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, toStatusError(err)
	}
	resp := &pb.LoadMoviesResponse{}
	for _, m := range res.Movies {
		resp.Movies = append(resp.Movies, toProto(m))
	}
	for _, e := range res.Errors {
		resp.Errors = append(resp.Errors, &pb.LoadError{Id: int32(e.ID), Error: e.Error})
	}
	return resp, nil
}

func (s *Server) GetLastMovie(ctx context.Context, _ *pb.GetLastMovieRequest) (*pb.Movie, error) {
	m, err := s.svc.GetLastMovie(ctx)
	if err != nil {
		return nil, toStatusError(err)
	}
	return toProto(m), nil
}

// toStatusError превращает ошибку обращения к стенду в gRPC-статус.
// Внутренние детали (URL и т.п.) клиенту не отдаём, только пишем в лог.
func toStatusError(err error) error {
	switch {
	case errors.Is(err, client.ErrNotFound):
		return status.Error(codes.NotFound, "no movies found on the stand")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "the movie stand did not respond in time")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "the request was canceled")
	case errors.Is(err, client.ErrBadResponse):
		log.Printf("stand error: %v", err)
		return status.Error(codes.Unavailable, "the movie stand returned an invalid response")
	default:
		log.Printf("stand error: %v", err)
		return status.Error(codes.Unavailable, "the movie stand is unavailable")
	}
}

func toProto(m *client.Movie) *pb.Movie {
	return &pb.Movie{
		Id:       int32(m.ID),
		Title:    m.Title,
		Year:     int32(m.Year),
		Director: m.Director,
	}
}
