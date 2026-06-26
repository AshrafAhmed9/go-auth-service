package grpcserver

import (
	"context"

	"github.com/AshrafAhmed9/assignment-golang/cache"
	"github.com/AshrafAhmed9/assignment-golang/proto/authpb"
	"github.com/AshrafAhmed9/assignment-golang/utils"
	"google.golang.org/grpc"
)

type AuthServer struct {
	authpb.UnimplementedAuthServiceServer
	jwtSecret   string
	redisClient *cache.RedisClient
}

func NewAuthServer(jwtSecret string, rdb *cache.RedisClient) *AuthServer {
	return &AuthServer{
		jwtSecret:   jwtSecret,
		redisClient: rdb,
	}
}

func (s *AuthServer) ValidateToken(ctx context.Context, req *authpb.ValidateTokenRequest) (*authpb.ValidateTokenResponse, error) {
	if req.Token == "" {
		return &authpb.ValidateTokenResponse{
			Valid: false,
			Error: "token is required",
		}, nil
	}

	claims, err := utils.ParseToken(req.Token, s.jwtSecret)
	if err != nil {
		return &authpb.ValidateTokenResponse{
			Valid: false,
			Error: "invalid or expired token",
		}, nil
	}

	if s.redisClient != nil && s.redisClient.IsBlacklisted(claims.ID) {
		return &authpb.ValidateTokenResponse{
			Valid: false,
			Error: "token has been revoked",
		}, nil
	}

	return &authpb.ValidateTokenResponse{
		Valid:  true,
		UserId: uint64(claims.UserID),
		Email:  claims.Email,
		Role:   claims.Role,
	}, nil
}

func RegisterServer(s *grpc.Server, jwtSecret string, rdb *cache.RedisClient) {
	authpb.RegisterAuthServiceServer(s, NewAuthServer(jwtSecret, rdb))
}
