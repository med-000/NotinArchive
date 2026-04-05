package pkg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func DebugSearchRaw() error {
	err := godotenv.Load()
	if err != nil {
		panic("failed to load .env")
	}

	url := "https://api.notion.com/v1/search"

	body := []byte(`{
		"page_size": 5
	}`)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+os.Getenv("NOTION_API_KEY"))
	req.Header.Set("Notion-Version", "2022-06-28")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	// ===== pretty整形 =====
	var pretty bytes.Buffer
	err = json.Indent(&pretty, raw, "", "  ")
	if err != nil {
		return err
	}

	// ===== ファイル保存 =====
	err = os.WriteFile("search_result.json", pretty.Bytes(), 0644)
	if err != nil {
		return err
	}

	fmt.Println("saved to search_result.json")

	return nil
}
