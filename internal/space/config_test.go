package space

import (
	"testing"
)

func TestStoreTypeConstants(t *testing.T) {
	if StoreTypeMemory != "memory" {
		t.Errorf("StoreTypeMemory = %q, want %q", StoreTypeMemory, "memory")
	}
	if StoreTypeSQLite != "sqlite" {
		t.Errorf("StoreTypeSQLite = %q, want %q", StoreTypeSQLite, "sqlite")
	}
	if StoreTypeDynamoDB != "dynamodb" {
		t.Errorf("StoreTypeDynamoDB = %q, want %q", StoreTypeDynamoDB, "dynamodb")
	}
}
