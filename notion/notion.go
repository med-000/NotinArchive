package notion

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type SearchResponse struct {
	Results []map[string]interface{} `json:"results"`
}

type Page struct {
	ID     string `json:"id"`
	Object string `json:"object"`

	Parent struct {
		Type string `json:"type"`
	} `json:"parent"`

	Properties struct {
		Title struct {
			Title []struct {
				PlainText string `json:"plain_text"`
			} `json:"title"`
		} `json:"title"`
	} `json:"properties"`
}

// =========================
// entry point
// =========================
func BuildFromSearch(rootDir string) error {
	res, err := search()
	if err != nil {
		return err
	}

	for _, raw := range res.Results {

		objType := raw["object"].(string)

		// =========================
		// PAGE
		// =========================
		if objType == "page" {
			b, _ := json.Marshal(raw)
			var p Page
			json.Unmarshal(b, &p)

			title := getTitle(p)
			dir := filepath.Join(rootDir, sanitize(title+"_"+shortID(p.ID)))

			fmt.Println("page:", dir)

			os.MkdirAll(dir, 0755)

			// 👇 pageは必ずmd作る
			createMD(dir, p.ID, title)

			walk(p.ID, dir, map[string]bool{})
		}

		// =========================
		// DATABASE
		// =========================
		if objType == "database" {
			b, _ := json.Marshal(raw)
			var d Database
			json.Unmarshal(b, &d)

			title := "database"
			if len(d.Title) > 0 {
				title = d.Title[0].PlainText
			}

			dir := filepath.Join(rootDir, sanitize(title+"_"+shortID(d.ID)))

			fmt.Println("db:", dir)

			os.MkdirAll(dir, 0755)

			// 👇 DBの中身取得
			rows, err := queryDatabase(d.ID)
			if err != nil {
				fmt.Println("db query err:", err)
				continue
			}

			for _, row := range rows {
				createMD(dir, row.ID, "row")
			}
		}
	}

	return nil
}

func walk(pageID string, currentPath string, visited map[string]bool) {
	if visited[pageID] {
		return
	}
	visited[pageID] = true

	children, err := getBlockChildren(pageID)
	if err != nil {
		fmt.Println("err:", err)
		return
	}

	hasChildPage := false

	for _, b := range children {
		if b.Type != "child_page" {
			continue
		}

		hasChildPage = true

		title := sanitize(b.ChildPage.Title)
		dir := filepath.Join(currentPath, title+"_"+shortID(b.ID))

		fmt.Println("mkdir:", dir)

		os.MkdirAll(dir, 0755)

		// 再帰
		walk(b.ID, dir, visited)
	}

	// 👇 子ページがない = leaf
	if !hasChildPage {
		filePath := filepath.Join(currentPath, shortID(pageID)+".md")

		fmt.Println("create file:", filePath)

		err := os.WriteFile(filePath, []byte("# "+pageID+"\n"), 0644)
		if err != nil {
			fmt.Println("write error:", err)
		}
	}
}

type BlockResponse struct {
	Results []Block `json:"results"`
}

type Block struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	ChildPage struct {
		Title string `json:"title"`
	} `json:"child_page"`
}
type Database struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	Title  []struct {
		PlainText string `json:"plain_text"`
	} `json:"title"`
}

func getBlockChildren(pageID string) ([]Block, error) {
	url := fmt.Sprintf("https://api.notion.com/v1/blocks/%s/children", pageID)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+os.Getenv("NOTION_API_KEY"))
	req.Header.Set("Notion-Version", "2022-06-28")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)

	var result BlockResponse
	json.Unmarshal(body, &result)

	return result.Results, nil
}

func search() (*SearchResponse, error) {
	url := "https://api.notion.com/v1/search"

	body := strings.NewReader(`{"page_size": 10}`)

	req, _ := http.NewRequest("POST", url, body)
	req.Header.Set("Authorization", "Bearer "+os.Getenv("NOTION_API_KEY"))
	req.Header.Set("Notion-Version", "2022-06-28")
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)

	var result SearchResponse
	json.Unmarshal(raw, &result)

	return &result, nil
}

func getTitle(p Page) string {
	if len(p.Properties.Title.Title) == 0 {
		return "untitled"
	}
	return p.Properties.Title.Title[0].PlainText
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

func shortID(id string) string {
	if len(id) < 6 {
		return id
	}
	return id[:6]
}
func queryDatabase(dbID string) ([]Page, error) {
	url := fmt.Sprintf("https://api.notion.com/v1/databases/%s/query", dbID)

	req, _ := http.NewRequest("POST", url, nil)
	req.Header.Set("Authorization", "Bearer "+os.Getenv("NOTION_API_KEY"))
	req.Header.Set("Notion-Version", "2022-06-28")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)

	var result struct {
		Results []Page `json:"results"`
	}

	json.Unmarshal(body, &result)

	return result.Results, nil
}

func createMD(dir, id, title string) {
	filePath := filepath.Join(dir, id+".md")

	content := fmt.Sprintf("# %s\n\nid: %s\n", title, id)

	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		fmt.Println("write error:", err)
	}
}
