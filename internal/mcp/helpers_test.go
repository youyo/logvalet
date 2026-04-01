package mcp

import "testing"

func TestBoolArg_True(t *testing.T) {
	args := map[string]any{"compact": true}
	v, ok := boolArg(args, "compact")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !v {
		t.Error("expected value=true")
	}
}

func TestBoolArg_False(t *testing.T) {
	args := map[string]any{"compact": false}
	v, ok := boolArg(args, "compact")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if v {
		t.Error("expected value=false")
	}
}

func TestBoolArg_Missing(t *testing.T) {
	args := map[string]any{}
	v, ok := boolArg(args, "compact")
	if ok {
		t.Error("expected ok=false for missing key")
	}
	if v {
		t.Error("expected value=false for missing key")
	}
}

func TestBoolArg_WrongType(t *testing.T) {
	args := map[string]any{"compact": "true"}
	v, ok := boolArg(args, "compact")
	if ok {
		t.Error("expected ok=false for wrong type")
	}
	if v {
		t.Error("expected value=false for wrong type")
	}
}
