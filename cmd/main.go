// cmd/main.go
package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/med-000/notionarchive/notion/service"
)

func main() {
	// ① .env 読み込み
	if err := godotenv.Load(); err != nil {
		log.Fatal("failed to load .env")
	}

	// ② 出力先（CLI引数 or デフォルト）
	rootDir := "./output"
	if len(os.Args) > 1 {
		rootDir = os.Args[1]
	}

	log.Println("start sync →", rootDir)

	// ③ 実行
	if err := service.Sync(rootDir); err != nil {
		log.Fatal(err)
	}

	log.Println("done")
}
