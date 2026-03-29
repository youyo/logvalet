package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// downloadToFile は io.ReadCloser のコンテンツを指定パスに書き込む。
// body は必ず Close される（defer）。
//
// outputPath が空の場合はカレントディレクトリに filename を使用して保存する。
// outputPath が指定されている場合は、親ディレクトリが存在しなければ os.MkdirAll で作成する。
// filename は filepath.Base() でサニタイズし、パストラバーサルを防止する。
// 書き込み途中でエラーが発生した場合は部分ファイルを os.Remove でクリーンアップする。
//
// 保存先のパスを返す。
func downloadToFile(body io.ReadCloser, filename string, outputPath string) (string, error) {
	defer body.Close()

	// filepath.Base でパストラバーサル防止
	safeName := filepath.Base(filename)
	if safeName == "." || safeName == "/" {
		safeName = "download"
	}

	var destPath string
	if outputPath == "" {
		destPath = safeName
	} else {
		// 親ディレクトリが存在しなければ作成
		dir := filepath.Dir(outputPath)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return "", fmt.Errorf("failed to create directory %q: %w", dir, err)
			}
		}
		destPath = outputPath
	}

	f, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file %q: %w", destPath, err)
	}

	if _, err := io.Copy(f, body); err != nil {
		f.Close()
		os.Remove(destPath)
		return "", fmt.Errorf("failed to write file %q: %w", destPath, err)
	}

	if err := f.Close(); err != nil {
		os.Remove(destPath)
		return "", fmt.Errorf("failed to close file %q: %w", destPath, err)
	}

	return destPath, nil
}
