package auth

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	testSecret  = []byte("test-secret-key-at-least-16bytes")
	testUserID  = "user-123"
	testTenant  = "example.backlog.com"
	testTTL     = 10 * time.Minute
	otherSecret = []byte("other-secret-key-at-least-16byte")
)

func TestGenerateState_ValidInput(t *testing.T) {
	state, err := GenerateState(testUserID, testTenant, testSecret, testTTL)
	if err != nil {
		t.Fatalf("GenerateState() error = %v, want nil", err)
	}
	if state == "" {
		t.Fatal("GenerateState() returned empty string")
	}
	// JWT は 3 パートで構成される
	parts := strings.Split(state, ".")
	if len(parts) != 3 {
		t.Fatalf("GenerateState() returned %d parts, want 3", len(parts))
	}
}

func TestGenerateState_EmptyUserID(t *testing.T) {
	_, err := GenerateState("", testTenant, testSecret, testTTL)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("GenerateState() error = %v, want ErrUnauthenticated", err)
	}
}

func TestGenerateState_EmptyTenant(t *testing.T) {
	_, err := GenerateState(testUserID, "", testSecret, testTTL)
	if !errors.Is(err, ErrInvalidTenant) {
		t.Fatalf("GenerateState() error = %v, want ErrInvalidTenant", err)
	}
}

func TestGenerateState_NilSecret(t *testing.T) {
	_, err := GenerateState(testUserID, testTenant, nil, testTTL)
	if !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("GenerateState() error = %v, want ErrStateInvalid", err)
	}
}

func TestGenerateState_EmptySecret(t *testing.T) {
	_, err := GenerateState(testUserID, testTenant, []byte{}, testTTL)
	if !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("GenerateState() error = %v, want ErrStateInvalid", err)
	}
}

func TestGenerateState_ZeroTTL(t *testing.T) {
	_, err := GenerateState(testUserID, testTenant, testSecret, 0)
	if !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("GenerateState() error = %v, want ErrStateInvalid", err)
	}
}

func TestGenerateState_NegativeTTL(t *testing.T) {
	_, err := GenerateState(testUserID, testTenant, testSecret, -5*time.Minute)
	if !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("GenerateState() error = %v, want ErrStateInvalid", err)
	}
}

func TestValidateState_RoundTrip(t *testing.T) {
	state, err := GenerateState(testUserID, testTenant, testSecret, testTTL)
	if err != nil {
		t.Fatalf("GenerateState() error = %v", err)
	}

	claims, err := ValidateState(state, testSecret)
	if err != nil {
		t.Fatalf("ValidateState() error = %v", err)
	}

	if claims.UserID != testUserID {
		t.Errorf("claims.UserID = %q, want %q", claims.UserID, testUserID)
	}
	if claims.Tenant != testTenant {
		t.Errorf("claims.Tenant = %q, want %q", claims.Tenant, testTenant)
	}
	if claims.Nonce == "" {
		t.Error("claims.Nonce is empty")
	}
	if claims.ExpiresAt == nil {
		t.Error("claims.ExpiresAt is nil")
	}
}

func TestValidateState_WrongSecret(t *testing.T) {
	state, err := GenerateState(testUserID, testTenant, testSecret, testTTL)
	if err != nil {
		t.Fatalf("GenerateState() error = %v", err)
	}

	_, err = ValidateState(state, otherSecret)
	if !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("ValidateState() error = %v, want ErrStateInvalid", err)
	}
}

func TestValidateState_ExpiredState(t *testing.T) {
	// 過去の ExpiresAt を持つ JWT を手動構築する（time.Sleep は使わない）
	claims := &StateClaims{
		UserID: testUserID,
		Tenant: testTenant,
		Nonce:  "test-nonce-expired",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	stateJWT, err := token.SignedString(testSecret)
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}

	_, err = ValidateState(stateJWT, testSecret)
	if !errors.Is(err, ErrStateExpired) {
		t.Fatalf("ValidateState() error = %v, want ErrStateExpired", err)
	}
}

func TestValidateState_TamperedPayload(t *testing.T) {
	state, err := GenerateState(testUserID, testTenant, testSecret, testTTL)
	if err != nil {
		t.Fatalf("GenerateState() error = %v", err)
	}

	// JWT の payload 部分を改竄する
	parts := strings.Split(state, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT parts, got %d", len(parts))
	}
	// payload をデコードして改竄
	tampered := base64.RawURLEncoding.EncodeToString([]byte(`{"uid":"attacker","tenant":"evil.backlog.com","nonce":"fake","exp":9999999999}`))
	tamperedJWT := parts[0] + "." + tampered + "." + parts[2]

	_, err = ValidateState(tamperedJWT, testSecret)
	if !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("ValidateState() error = %v, want ErrStateInvalid", err)
	}
}

func TestValidateState_EmptyString(t *testing.T) {
	_, err := ValidateState("", testSecret)
	if !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("ValidateState() error = %v, want ErrStateInvalid", err)
	}
}

func TestGenerateState_NonceUniqueness(t *testing.T) {
	state1, err := GenerateState(testUserID, testTenant, testSecret, testTTL)
	if err != nil {
		t.Fatalf("GenerateState() #1 error = %v", err)
	}
	state2, err := GenerateState(testUserID, testTenant, testSecret, testTTL)
	if err != nil {
		t.Fatalf("GenerateState() #2 error = %v", err)
	}

	claims1, err := ValidateState(state1, testSecret)
	if err != nil {
		t.Fatalf("ValidateState() #1 error = %v", err)
	}
	claims2, err := ValidateState(state2, testSecret)
	if err != nil {
		t.Fatalf("ValidateState() #2 error = %v", err)
	}

	if claims1.Nonce == claims2.Nonce {
		t.Errorf("nonce should be unique, got same nonce: %q", claims1.Nonce)
	}
}

func TestValidateState_AlgorithmNone(t *testing.T) {
	// alg:none 攻撃を模擬: 署名なし JWT を構築
	claims := &StateClaims{
		UserID: testUserID,
		Tenant: testTenant,
		Nonce:  "attack-nonce",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	stateJWT, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}

	_, err = ValidateState(stateJWT, testSecret)
	if !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("ValidateState() error = %v, want ErrStateInvalid", err)
	}
}
