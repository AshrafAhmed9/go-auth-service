package main

// ValidateToken is the one endpoint the Spring Boot service calls, over
// gRPC, on every request it handles. It never returns a gRPC error for a
// bad token — invalid, expired and revoked tokens all come back as
// valid=false with a reason, because "the token is bad" is an expected
// outcome, not a transport failure. Only a real transport problem (the
// server being unreachable) should look like an error to the caller.
import (
	"context"

	"github.com/AshrafAhmed9/assignment-golang/proto/authpb"
	"google.golang.org/grpc"
)

type authServer struct {
	authpb.UnimplementedAuthServiceServer
	jwtSecret string
	cache     *RedisClient
}

func registerGRPCServer(s *grpc.Server, jwtSecret string, cache *RedisClient) {
	authpb.RegisterAuthServiceServer(s, &authServer{jwtSecret: jwtSecret, cache: cache})
}

func (s *authServer) ValidateToken(ctx context.Context, req *authpb.ValidateTokenRequest) (*authpb.ValidateTokenResponse, error) {
	if req.Token == "" {
		return &authpb.ValidateTokenResponse{Valid: false, Error: "token is required"}, nil
	}

	claims, err := parseToken(req.Token, s.jwtSecret)
	if err != nil {
		return &authpb.ValidateTokenResponse{Valid: false, Error: "invalid or expired token"}, nil
	}

	if s.cache != nil && s.cache.IsBlacklisted(claims.ID) {
		return &authpb.ValidateTokenResponse{Valid: false, Error: "token has been revoked"}, nil
	}

	return &authpb.ValidateTokenResponse{
		Valid:  true,
		UserId: uint64(claims.UserID),
		Email:  claims.Email,
		Role:   claims.Role,
	}, nil
}
