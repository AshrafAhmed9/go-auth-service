package main

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/AshrafAhmed9/assignment-golang/proto/authpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func startTestGRPCServer(t *testing.T) authpb.AuthServiceClient {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	srv := grpc.NewServer()
	registerGRPCServer(srv, testSecret, nil)
	go srv.Serve(lis)
	t.Cleanup(srv.GracefulStop)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return authpb.NewAuthServiceClient(conn)
}

func TestGRPCValidateToken_Valid(t *testing.T) {
	client := startTestGRPCServer(t)
	token, _ := generateToken(1, "alice@test.com", "user", testSecret, 15*time.Minute)

	resp, err := client.ValidateToken(context.Background(), &authpb.ValidateTokenRequest{Token: token})
	if err != nil {
		t.Fatalf("gRPC call failed: %v", err)
	}
	if !resp.Valid || resp.UserId != 1 || resp.Email != "alice@test.com" || resp.Role != "user" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestGRPCValidateToken_Expired(t *testing.T) {
	client := startTestGRPCServer(t)
	token, _ := generateToken(1, "alice@test.com", "user", testSecret, -time.Second)

	resp, err := client.ValidateToken(context.Background(), &authpb.ValidateTokenRequest{Token: token})
	if err != nil {
		t.Fatalf("gRPC call failed: %v", err)
	}
	if resp.Valid || resp.Error == "" {
		t.Errorf("expected valid=false with a reason for an expired token, got %+v", resp)
	}
}

func TestGRPCValidateToken_Empty(t *testing.T) {
	client := startTestGRPCServer(t)

	resp, err := client.ValidateToken(context.Background(), &authpb.ValidateTokenRequest{Token: ""})
	if err != nil {
		t.Fatalf("gRPC call failed: %v", err)
	}
	if resp.Valid {
		t.Error("expected valid=false for an empty token")
	}
}

func TestGRPCValidateToken_Garbage(t *testing.T) {
	client := startTestGRPCServer(t)

	resp, err := client.ValidateToken(context.Background(), &authpb.ValidateTokenRequest{Token: "garbage.token.here"})
	if err != nil {
		t.Fatalf("gRPC call failed: %v", err)
	}
	if resp.Valid {
		t.Error("expected valid=false for a garbage token")
	}
}
