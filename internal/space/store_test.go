package space

import (
	"errors"
	"testing"
)

// S23 決定F: HTTP/Gateway モードでは LOGVALET_SPACE_STORE_TYPE の明示指定を
// 必須とする。未設定・memory 選択時は ErrSpaceStoreTypeRequired を返す。
// sqlite/dynamodb は明示指定として許容される。
func TestRequireExplicitStoreType(t *testing.T) {
	cases := []struct {
		name      string
		storeType string
		wantErr   bool
	}{
		{"empty", "", true},
		{"whitespace_only", "   ", true},
		{"memory", "memory", true},
		{"memory_uppercase", "MEMORY", true},
		{"memory_mixed_case_with_space", " Memory ", true},
		{"sqlite", "sqlite", false},
		{"dynamodb", "dynamodb", false},
		{"sqlite_uppercase", "SQLITE", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := RequireExplicitStoreType(tc.storeType)
			if tc.wantErr {
				if !errors.Is(err, ErrSpaceStoreTypeRequired) {
					t.Errorf("RequireExplicitStoreType(%q) = %v, want ErrSpaceStoreTypeRequired", tc.storeType, err)
				}
			} else if err != nil {
				t.Errorf("RequireExplicitStoreType(%q) = %v, want nil", tc.storeType, err)
			}
		})
	}
}
