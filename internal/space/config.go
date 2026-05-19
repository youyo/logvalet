package space

import "fmt"

type StoreType string

const (
	StoreTypeMemory   StoreType = "memory"
	StoreTypeSQLite   StoreType = "sqlite"
	StoreTypeDynamoDB StoreType = "dynamodb"
)

// ValidateSpaceStoreConfig は store type と remote MCP モードの組み合わせを検証する。
// remote MCP で SQLite または memory を使うと user_id="local" 固定で
// multi-tenant 漏洩が発生するため dynamodb のみ許可する（C1 対応）。
func ValidateSpaceStoreConfig(storeType string, isMCPRemote bool) error {
	switch StoreType(storeType) {
	case StoreTypeMemory, StoreTypeSQLite, StoreTypeDynamoDB:
		// 有効な store type
	default:
		return fmt.Errorf(
			"space: invalid store type %q; must be one of: memory, sqlite, dynamodb",
			storeType,
		)
	}

	if isMCPRemote && StoreType(storeType) != StoreTypeDynamoDB {
		return fmt.Errorf(
			"remote MCP requires dynamodb store type (got %q). "+
				"Set LOGVALET_SPACE_STORE_TYPE=dynamodb",
			storeType,
		)
	}
	return nil
}
