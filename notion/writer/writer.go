package writer

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

func CreateDir(path string) {
	os.MkdirAll(path, 0755)
}

func WriteMD(dir, id, content string) {
	filePath := filepath.Join(dir, id+".md")
	os.WriteFile(filePath, []byte(content), 0644)
}

func WriteMDStem(dir, stem, content string) {
	filePath := filepath.Join(dir, stem+".md")
	os.WriteFile(filePath, []byte(content), 0644)
}

func Sanitize(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if s == "" {
		return "untitled"
	}
	return s
}

func ShortID(id string) string {
	if len(id) < 6 {
		return id
	}
	return id[:6]
}

func UniqueIDStem(id string, used map[string]string) string {
	if stem, ok := used[id]; ok {
		return stem
	}

	lengths := []int{8, 12, 16, len(id)}
	for _, n := range lengths {
		if n > len(id) {
			n = len(id)
		}

		stem := id[:n]
		if isStemAvailable(stem, id, used) {
			used[id] = stem
			return stem
		}
	}

	used[id] = id
	return id
}

func isStemAvailable(stem, id string, used map[string]string) bool {
	for existingID, existingStem := range used {
		if existingID != id && existingStem == stem {
			return false
		}
	}
	return true
}
