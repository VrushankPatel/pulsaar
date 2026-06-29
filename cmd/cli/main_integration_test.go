package main

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAccessErrorFormatting(t *testing.T) {
	err1 := &AccessError{
		Type:    "auth",
		Message: "unauthorized request",
		Detail:  "token expired",
	}
	expected1 := "unauthorized request (Type: auth). Hint: Check your kubeconfig and token validity. Detail: token expired"
	if err1.Error() != expected1 {
		t.Errorf("expected: %q, got: %q", expected1, err1.Error())
	}

	err2 := &AccessError{
		Type:    "rbac",
		Message: "rbac failure",
	}
	expected2 := "rbac failure (Type: rbac). Hint: Ask cluster admin to grant 'get pods' in this namespace"
	if err2.Error() != expected2 {
		t.Errorf("expected: %q, got: %q", expected2, err2.Error())
	}
}

func TestWrapgRPCError(t *testing.T) {
	// 1. PermissionDenied
	gErr1 := status.Error(codes.PermissionDenied, "internal permission restriction details")
	wErr1 := wrapgRPCError(gErr1, "/etc/passwd")
	acErr1, ok := wErr1.(*AccessError)
	if !ok {
		t.Fatalf("expected AccessError, got %T", wErr1)
	}
	if acErr1.Type != "path" {
		t.Errorf("expected Type path, got %s", acErr1.Type)
	}
	if acErr1.Detail != "internal permission restriction details" {
		t.Errorf("expected detail to match gRPC message, got %s", acErr1.Detail)
	}

	// 2. ResourceExhausted
	gErr2 := status.Error(codes.ResourceExhausted, "rate limit exceeded message")
	wErr2 := wrapgRPCError(gErr2, "/var/log")
	acErr2, ok := wErr2.(*AccessError)
	if !ok {
		t.Fatalf("expected AccessError, got %T", wErr2)
	}
	if acErr2.Type != "connectivity" {
		t.Errorf("expected Type connectivity, got %s", acErr2.Type)
	}

	// 3. Raw generic error
	rawErr := errors.New("raw connection failure")
	wErr3 := wrapgRPCError(rawErr, "")
	acErr3, ok := wErr3.(*AccessError)
	if !ok {
		t.Fatalf("expected AccessError, got %T", wErr3)
	}
	if acErr3.Type != "connectivity" {
		t.Errorf("expected Type connectivity, got %s", acErr3.Type)
	}
}
