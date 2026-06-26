package tests

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/AshrafAhmed9/assignment-golang/grpcserver"
	"github.com/AshrafAhmed9/assignment-golang/proto/authpb"
	"github.com/AshrafAhmed9/assignment-golang/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func startTestGRPCServer(t *testing.T, secret string) (authpb.AuthServiceClient, func()) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal("failed to listen:", err)
	}

	srv := grpc.NewServer()
	grpcserver.RegisterServer(srv, secret, nil)

	go srv.Serve(lis)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal("failed to dial:", err)
	}

	client := authpb.NewAuthServiceClient(conn)
	cleanup := func() {
		conn.Close()
		srv.GracefulStop()
	}

	return client, cleanup
}

func TestGRPC_ValidateToken_Valid(t *testing.T) {
	secret := "test-secret-key-that-is-32-chars!!"
	client, cleanup := startTestGRPCServer(t, secret)
	defer cleanup()

	token, _ := utils.GenerateToken(1, "alice@test.com", "user", secret, 15*time.Minute)

	resp, err := client.ValidateToken(context.Background(), &authpb.ValidateTokenRequest{Token: token})
	if err != nil {
		t.Fatal("gRPC call failed:", err)
	}

	if !resp.Valid {
		t.Errorf("expected valid=true, got false: %s", resp.Error)
	}
	if resp.UserId != 1 {
		t.Errorf("expected user_id=1, got %d", resp.UserId)
	}
	if resp.Email != "alice@test.com" {
		t.Errorf("expected email=alice@test.com, got %s", resp.Email)
	}
	if resp.Role != "user" {
		t.Errorf("expected role=user, got %s", resp.Role)
	}
}

func TestGRPC_ValidateToken_Expired(t *testing.T) {
	secret := "test-secret-key-that-is-32-chars!!"
	client, cleanup := startTestGRPCServer(t, secret)
	defer cleanup()

	token, _ := utils.GenerateToken(1, "alice@test.com", "user", secret, -1*time.Second)

	resp, err := client.ValidateToken(context.Background(), &authpb.ValidateTokenRequest{Token: token})
	if err != nil {
		t.Fatal("gRPC call failed:", err)
	}

	if resp.Valid {
		t.Error("expected valid=false for expired token")
	}
	if resp.Error == "" {
		t.Error("expected error message for expired token")
	}
}

func TestGRPC_ValidateToken_Empty(t *testing.T) {
	secret := "test-secret-key-that-is-32-chars!!"
	client, cleanup := startTestGRPCServer(t, secret)
	defer cleanup()

	resp, err := client.ValidateToken(context.Background(), &authpb.ValidateTokenRequest{Token: ""})
	if err != nil {
		t.Fatal("gRPC call failed:", err)
	}

	if resp.Valid {
		t.Error("expected valid=false for empty token")
	}
}

func TestGRPC_ValidateToken_Invalid(t *testing.T) {
	secret := "test-secret-key-that-is-32-chars!!"
	client, cleanup := startTestGRPCServer(t, secret)
	defer cleanup()

	resp, err := client.ValidateToken(context.Background(), &authpb.ValidateTokenRequest{Token: "garbage.token.here"})
	if err != nil {
		t.Fatal("gRPC call failed:", err)
	}

	if resp.Valid {
		t.Error("expected valid=false for invalid token")
	}
}
