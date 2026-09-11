package conventions

import "strings"

// StatusColors は状態に指定できる色。
var StatusColors = []string{
	"#ea2c00", "#e87758", "#e07b9a", "#868cb7", "#3b9dbd",
	"#4caf93", "#b0be3c", "#eda62a", "#f42858", "#393939",
}

// IssueTypeColors は課題種別に指定できる色。
var IssueTypeColors = []string{
	"#e30000", "#990000", "#934981", "#814fbc", "#2779ca",
	"#007e9a", "#7ea800", "#ff9200", "#ff3265", "#666665",
}

// IsValidStatusColor は状態用 allowlist に含まれる色かを返す。
func IsValidStatusColor(s string) bool {
	return isValidColor(StatusColors, s)
}

// IsValidIssueTypeColor は課題種別用 allowlist に含まれる色かを返す。
func IsValidIssueTypeColor(s string) bool {
	return isValidColor(IssueTypeColors, s)
}

func isValidColor(colors []string, value string) bool {
	value = strings.ToLower(value)
	for _, color := range colors {
		if strings.ToLower(color) == value {
			return true
		}
	}
	return false
}
