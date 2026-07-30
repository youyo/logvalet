package cli

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/youyo/logvalet/internal/auth"
)

// identity ヘッダー名。docs/specs/gateway-request-contract.md §2.2 で確定した契約。
// iss + sub の組で一意性を担保するため 2 ヘッダーに分離している。
const (
	identityIssuerHeaderName  = "X-Logvalet-Identity-Issuer"
	identitySubjectHeaderName = "X-Logvalet-Identity-Subject"
)

// maxIdentityValueLen は identity ヘッダー 1 値あたりの最大バイト長。
// Entra ID の issuer URL / sub claim はいずれもこの長さに十分収まる。
const maxIdentityValueLen = 512

// identityMiddleware は Gateway が注入した identity ヘッダーから userID を導出し、
// auth.ContextWithUserID で context へ注入する HTTP ミドルウェア。
//
// **apikey 検証を通過したリクエストでのみ identity を信用する**（契約 §2.3）ため、
// 必ず apiKeyAuthMiddleware の内側に配線する。apikey 不成立のリクエストは
// このミドルウェアに到達せず、ヘッダー値は一切参照されない。
//
// strip-and-replace（契約 §2.4）: クライアント由来の同名ヘッダーが混入していても
// ハンドラーへは渡さない。リクエストのクローンから両ヘッダーを削除した上で、
// 検証済みの userID だけを context に載せて次のハンドラーへ渡す。
//
// 単一テナント固定 userID へのフォールバックは行わない（決定D）。identity が
// 確立できないリクエストはエラーエンベロープで拒否する。契約 §2.3 は identity 欠落を
// 「UserIDFromContext が ok=false を返す状態」として通す余地も残しているが、
// 複数ユーザー前提（決定D）では identity 無しのリクエストは Gateway 設定ミスであり、
// ツール個別のエラーまで遅らせず入口で fail-closed にする。
func identityMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			issuer, err := identityHeaderValue(r, identityIssuerHeaderName)
			if err != nil {
				writeIdentityError(w, err)
				return
			}
			subject, err := identityHeaderValue(r, identitySubjectHeaderName)
			if err != nil {
				writeIdentityError(w, err)
				return
			}

			ctx := auth.ContextWithUserID(r.Context(), auth.NormalizeUserID(issuer, subject))
			next.ServeHTTP(w, stripIdentityHeaders(r.Clone(ctx)))
		})
	}
}

// stripIdentityMiddleware は identity を注入せず、クライアント由来の identity ヘッダーを
// 落とすだけのミドルウェア。auth-mode=none 用（契約 §2.3: apikey 検証を通過していない
// identity は信用しない。none は開発・信頼済みネットワーク限定の運用）。
func stripIdentityMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, stripIdentityHeaders(r.Clone(r.Context())))
		})
	}
}

func stripIdentityHeaders(r *http.Request) *http.Request {
	r.Header.Del(identityIssuerHeaderName)
	r.Header.Del(identitySubjectHeaderName)
	return r
}

// identityError は identity 検証の失敗を HTTP ステータス・エラーコード付きで表す。
type identityError struct {
	status  int
	code    string
	message string
}

func (e *identityError) Error() string { return e.message }

// identityHeaderValue は 1 本の identity ヘッダーを取り出して検証する。
func identityHeaderValue(r *http.Request, name string) (string, error) {
	values := r.Header.Values(name)
	switch {
	case len(values) == 0 || (len(values) == 1 && values[0] == ""):
		return "", &identityError{
			status:  http.StatusUnauthorized,
			code:    "identity_required",
			message: fmt.Sprintf("Missing %s header. The gateway must inject the verified end-user identity.", name),
		}
	case len(values) > 1:
		// 同名ヘッダーが複数ある場合、どれが Gateway 由来かを区別できない
		// （クライアント値に Gateway 値が append された可能性を排除できない）。
		return "", &identityError{
			status:  http.StatusBadRequest,
			code:    "invalid_identity",
			message: fmt.Sprintf("Duplicate %s header: the gateway must strip client-supplied values and set exactly one.", name),
		}
	}

	v := values[0]
	if len(v) > maxIdentityValueLen {
		return "", &identityError{
			status:  http.StatusBadRequest,
			code:    "invalid_identity",
			message: fmt.Sprintf("%s exceeds the maximum length of %d bytes.", name, maxIdentityValueLen),
		}
	}
	if !isValidIdentityValue(v) {
		return "", &identityError{
			status:  http.StatusBadRequest,
			code:    "invalid_identity",
			message: fmt.Sprintf("%s contains characters outside the allowed set (printable US-ASCII, no space or comma).", name),
		}
	}
	return v, nil
}

// isValidIdentityValue は identity ヘッダー値の文字種を検証する。
// 許可するのは空白・カンマを除く印字可能な US-ASCII のみ。issuer URL も Entra ID の
// sub claim もこの範囲に収まる。カンマを弾くのは、1 行のヘッダーに複数値を詰めて
// 送る形の偽装（`iss1,iss2`）を重複ヘッダーと同様に拒否するため。
func isValidIdentityValue(v string) bool {
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c <= 0x20 || c >= 0x7f || c == ',' {
			return false
		}
	}
	return true
}

// identityErrorEnvelope は spec §9 のエラーエンベロープ（HTTP レスポンスボディ用）。
type identityErrorEnvelope struct {
	SchemaVersion string `json:"schema_version"`
	Error         struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		Retryable bool   `json:"retryable"`
	} `json:"error"`
}

// writeIdentityError は spec §9 のエラーエンベロープ形式で失敗を返す。
func writeIdentityError(w http.ResponseWriter, err error) {
	ie, ok := err.(*identityError)
	if !ok {
		ie = &identityError{status: http.StatusBadRequest, code: "invalid_identity", message: err.Error()}
	}

	var env identityErrorEnvelope
	env.SchemaVersion = "1"
	env.Error.Code = ie.code
	env.Error.Message = ie.message

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(ie.status)
	if err := json.NewEncoder(w).Encode(env); err != nil {
		slog.Warn("failed to write identity error envelope", "error", err)
	}
}
