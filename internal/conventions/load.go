package conventions

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load はバイト列を Conventions にデコードする。未知フィールドはエラーにする。
func Load(data []byte) (*Conventions, error) {
	var conventions Conventions
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&conventions); err != nil {
		return nil, fmt.Errorf("conventions YAML のデコードに失敗しました: %w", err)
	}
	return &conventions, nil
}

// LoadFile はファイルパスから読む。
func LoadFile(path string) (*Conventions, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("conventions ファイルの読み込みに失敗しました: %w", err)
	}
	return Load(data)
}

// LoadFromIssueDescription は規約課題の説明欄から YAML コードブロックを取り出してデコードする。
func LoadFromIssueDescription(description string) (*Conventions, error) {
	block, ok := lastYAMLBlock(description)
	if !ok {
		return nil, fmt.Errorf("規約課題の説明欄に yaml コードブロックがありません")
	}
	return Load([]byte(block))
}

func lastYAMLBlock(description string) (string, bool) {
	var (
		inBlock bool
		lines   []string
		last    string
		found   bool
	)

	for _, line := range strings.Split(description, "\n") {
		line = strings.TrimSuffix(line, "\r")
		marker := strings.TrimSpace(line)
		if !inBlock {
			if marker == "```yaml" {
				inBlock = true
				lines = nil
			}
			continue
		}
		if marker == "```" {
			last = strings.Join(lines, "\n")
			found = true
			inBlock = false
			lines = nil
			continue
		}
		lines = append(lines, line)
	}

	return last, found
}
