// cmd/main.go
package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	githubservice "github.com/med-000/notionarchive/github/service"
	"github.com/med-000/notionarchive/notion/service"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("failed to load .env")
	}

	if len(os.Args) > 1 && os.Args[1] == "issues" {
		log.Println("start issue sync")
		if err := githubservice.SyncIssues(); err != nil {
			log.Fatal(err)
		}
		log.Println("done")
		return
	}

	rootDir := "./output"
	if len(os.Args) > 1 {
		rootDir = os.Args[1]
	}

	log.Println("start sync →", rootDir)

	if err := service.Sync(rootDir); err != nil {
		log.Fatal(err)
	}

	log.Println("done")
}
