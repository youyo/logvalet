package conventions

import "testing"

func TestIsValidStatusColor(t *testing.T) {
	tests := []struct {
		name  string
		color string
		want  bool
	}{
		{name: "allowlist", color: StatusColors[0], want: true},
		{name: "case insensitive", color: "#EA2C00", want: true},
		{name: "not allowlist", color: "#ffffff", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidStatusColor(tt.color); got != tt.want {
				t.Fatalf("IsValidStatusColor(%q) = %t, want %t", tt.color, got, tt.want)
			}
		})
	}
}

func TestIsValidIssueTypeColor(t *testing.T) {
	if !IsValidIssueTypeColor("#E30000") {
		t.Fatal("大文字の種別色を有効と判定できませんでした")
	}
	if IsValidIssueTypeColor("#ffffff") {
		t.Fatal("allowlist 外の種別色を有効と判定しました")
	}
}
