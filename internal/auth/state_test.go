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

// ============================================================================
// S1-S8: GenerateStateWithContinue / ValidateContinueURL テスト
// ============================================================================

// S1: GenerateStateWithContinue でラウンドトリップし、Continue フィールドが保持されること。
func TestGenerateStateWithContinue_RoundTrip(t *testing.T) {
	continueURL := "/authorize?x=1"
	state, err := GenerateStateWithContinue(testUserID, testTenant, continueURL, testSecret, testTTL)
	if err != nil {
		t.Fatalf("GenerateStateWithContinue() error = %v, want nil", err)
	}
	claims, err := ValidateState(state, testSecret)
	if err != nil {
		t.Fatalf("ValidateState() error = %v, want nil", err)
	}
	if claims.Continue != continueURL {
		t.Errorf("claims.Continue = %q, want %q", claims.Continue, continueURL)
	}
	if claims.UserID != testUserID {
		t.Errorf("claims.UserID = %q, want %q", claims.UserID, testUserID)
	}
}

// S2: 既存 GenerateState は Continue フィールドが空で後方互換を維持すること。
func TestGenerateState_BackwardCompatNoContinue(t *testing.T) {
	state, err := GenerateState(testUserID, testTenant, testSecret, testTTL)
	if err != nil {
		t.Fatalf("GenerateState() error = %v, want nil", err)
	}
	claims, err := ValidateState(state, testSecret)
	if err != nil {
		t.Fatalf("ValidateState() error = %v, want nil", err)
	}
	if claims.Continue != "" {
		t.Errorf("claims.Continue = %q, want empty string (backward compat)", claims.Continue)
	}
}

// S3: ValidateContinueURL("/authorize?a=b") は nil を返すこと（/authorize prefix 有効）。
func TestValidateContinueURL_ValidAuthorizePrefix(t *testing.T) {
	if err := ValidateContinueURL("/authorize?a=b"); err != nil {
		t.Errorf("ValidateContinueURL(%q) = %v, want nil", "/authorize?a=b", err)
	}
}

// S4: ValidateContinueURL("https://evil.example/x") は ErrInvalidContinue を返すこと。
func TestValidateContinueURL_AbsoluteURLRejected(t *testing.T) {
	if err := ValidateContinueURL("https://evil.example/x"); !errors.Is(err, ErrInvalidContinue) {
		t.Errorf("ValidateContinueURL(%q) = %v, want ErrInvalidContinue", "https://evil.example/x", err)
	}
}

// S5: ValidateContinueURL("//evil.example/x") は ErrInvalidContinue を返すこと（protocol-relative）。
func TestValidateContinueURL_ProtocolRelativeRejected(t *testing.T) {
	if err := ValidateContinueURL("//evil.example/x"); !errors.Is(err, ErrInvalidContinue) {
		t.Errorf("ValidateContinueURL(%q) = %v, want ErrInvalidContinue", "//evil.example/x", err)
	}
}

// S6: ValidateContinueURL("\\\\evil") は ErrInvalidContinue を返すこと（backslash）。
func TestValidateContinueURL_BackslashRejected(t *testing.T) {
	if err := ValidateContinueURL(`\\evil`); !errors.Is(err, ErrInvalidContinue) {
		t.Errorf("ValidateContinueURL(%q) = %v, want ErrInvalidContinue", `\\evil`, err)
	}
}

// S7: ValidateContinueURL("/anything") は ErrInvalidContinue を返すこと（/authorize prefix 以外）。
func TestValidateContinueURL_NonAuthorizePrefixRejected(t *testing.T) {
	if err := ValidateContinueURL("/anything"); !errors.Is(err, ErrInvalidContinue) {
		t.Errorf("ValidateContinueURL(%q) = %v, want ErrInvalidContinue", "/anything", err)
	}
}

// S8: ValidateContinueURL("") は nil を返すこと（空は「継続先なし」として許容）。
func TestValidateContinueURL_EmptyAllowed(t *testing.T) {
	if err := ValidateContinueURL(""); err != nil {
		t.Errorf("ValidateContinueURL(%q) = %v, want nil", "", err)
	}
}
