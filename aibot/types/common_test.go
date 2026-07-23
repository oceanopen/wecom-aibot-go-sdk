package types

import (
	"errors"
	"testing"
)

func TestWsAuthFailureError(t *testing.T) {
	err := NewWsAuthFailureError(5)

	// errors.As 可识别为 *WsAuthFailureError
	var target *WsAuthFailureError
	if !errors.As(err, &target) {
		t.Fatalf("errors.As should match *WsAuthFailureError")
	}
	if target.MaxAttempts != 5 {
		t.Fatalf("MaxAttempts = %d, want 5", target.MaxAttempts)
	}

	// Code 返回稳定码
	if got := target.Code(); got != WsAuthFailureCode {
		t.Fatalf("Code() = %q, want %q", got, WsAuthFailureCode)
	}

	// Error 信息包含次数
	if got := target.Error(); got != "Max auth failure attempts exceeded (5)" {
		t.Fatalf("Error() = %q, want exact match", got)
	}
}

func TestWsReconnectExhaustedError(t *testing.T) {
	err := NewWsReconnectExhaustedError(10)

	// errors.As 可识别为 *WsReconnectExhaustedError
	var target *WsReconnectExhaustedError
	if !errors.As(err, &target) {
		t.Fatalf("errors.As should match *WsReconnectExhaustedError")
	}
	if target.MaxAttempts != 10 {
		t.Fatalf("MaxAttempts = %d, want 10", target.MaxAttempts)
	}

	// Code 返回稳定码
	if got := target.Code(); got != WsReconnectExhaustedCode {
		t.Fatalf("Code() = %q, want %q", got, WsReconnectExhaustedCode)
	}

	// Error 信息包含次数
	if got := target.Error(); got != "Max reconnect attempts exceeded (10)" {
		t.Fatalf("Error() = %q, want exact match", got)
	}
}

func TestErrorCodesAreDistinct(t *testing.T) {
	auth := NewWsAuthFailureError(1)
	reconn := NewWsReconnectExhaustedError(1)

	if auth.Code() == reconn.Code() {
		t.Fatalf("auth and reconnect error codes should differ, both = %q", auth.Code())
	}

	// errors.As 互不串扰
	var authTarget *WsAuthFailureError
	if errors.As(reconn, &authTarget) {
		t.Fatalf("reconnect error must not match *WsAuthFailureError")
	}
	var reconnTarget *WsReconnectExhaustedError
	if errors.As(auth, &reconnTarget) {
		t.Fatalf("auth error must not match *WsReconnectExhaustedError")
	}
}
