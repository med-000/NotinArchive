package writer

import (
	"os"
	"path/filepath"
	"strings"
)

func CreateDir(path string) {
	os.MkdirAll(path, 0755)
}

func WriteMD(dir, id, content string) {
	filePath := filepath.Join(dir, id+".md")
	os.WriteFile(filePath, []byte(content), 0644)
}

func Sanitize(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

func ShortID(id string) string {
	if len(id) < 6 {
		return id
	}
	return id[:6]
}
