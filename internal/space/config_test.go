package space

import (
	"strings"
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

func TestValidateSpaceStoreConfig_Remote_Sqlite_Rejected(t *testing.T) {
	err := ValidateSpaceStoreConfig("sqlite", true)
	if err == nil {
		t.Fatal("expected error for remote MCP + sqlite, got nil")
	}
	if !strings.Contains(err.Error(), "remote MCP requires dynamodb") {
		t.Errorf("error message %q does not contain 'remote MCP requires dynamodb'", err.Error())
	}
}

func TestValidateSpaceStoreConfig_Remote_Memory_Rejected(t *testing.T) {
	err := ValidateSpaceStoreConfig("memory", true)
	if err == nil {
		t.Fatal("expected error for remote MCP + memory, got nil")
	}
	if !strings.Contains(err.Error(), "remote MCP requires dynamodb") {
		t.Errorf("error message %q does not contain 'remote MCP requires dynamodb'", err.Error())
	}
}

func TestValidateSpaceStoreConfig_Remote_DynamoDB_OK(t *testing.T) {
	err := ValidateSpaceStoreConfig("dynamodb", true)
	if err != nil {
		t.Errorf("expected nil for remote MCP + dynamodb, got %v", err)
	}
}

func TestValidateSpaceStoreConfig_Local_Sqlite_OK(t *testing.T) {
	err := ValidateSpaceStoreConfig("sqlite", false)
	if err != nil {
		t.Errorf("expected nil for local + sqlite, got %v", err)
	}
}

func TestValidateSpaceStoreConfig_Local_Memory_OK(t *testing.T) {
	err := ValidateSpaceStoreConfig("memory", false)
	if err != nil {
		t.Errorf("expected nil for local + memory, got %v", err)
	}
}

func TestValidateSpaceStoreConfig_InvalidStoreType(t *testing.T) {
	err := ValidateSpaceStoreConfig("invalid", false)
	if err == nil {
		t.Fatal("expected error for invalid store type, got nil")
	}
}
