package walker

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/med-000/notionarchive/notion/client"
	"github.com/med-000/notionarchive/notion/converter"
	"github.com/med-000/notionarchive/notion/model"
	"github.com/med-000/notionarchive/notion/writer"
)

func Walk(c *client.Client, pageID, currentPath, pageTitle string, visited map[string]bool) {
	if visited[pageID] {
		return
	}
	visited[pageID] = true

	rawBlocks, err := c.GetBlocks(pageID)
	if err != nil {
		fmt.Println("err:", err)
		return
	}

	hasChild := false

	for _, raw := range rawBlocks {
		if isArchivedOrTrashed(raw) {
			continue
		}

		b, _ := json.Marshal(raw)
		var block model.Block
		json.Unmarshal(b, &block)

		switch block.Type {
		case "child_page":
			hasChild = true

			title := writer.Sanitize(block.ChildPage.Title)
			dir := filepath.Join(currentPath, title)

			writer.CreateDir(dir)
			Walk(c, block.ID, dir, block.ChildPage.Title, visited)

		case "child_database":
			hasChild = true

			title := writer.Sanitize(block.ChildDatabase.Title)
			dir := filepath.Join(currentPath, title)

			writer.CreateDir(dir)

			if err := exportDatabaseRows(c, block.ID, dir); err != nil {
				if client.IsInaccessibleDatabaseError(err) {
					fmt.Println("skip inaccessible db:", block.ID)
					continue
				}
				fmt.Println("db err:", err)
				continue
			}
		}
	}

	if !hasChild {
		blocks, err := c.GetAllBlocks(pageID)
		if err != nil {
			return
		}

		md := converter.BlocksToMarkdown(pageTitle, blocks)

		writer.WriteMD(currentPath, pageID, md)
	}
}

func isArchivedOrTrashed(raw map[string]interface{}) bool {
	if value, ok := raw["in_trash"].(bool); ok && value {
		return true
	}

	if value, ok := raw["archived"].(bool); ok && value {
		return true
	}

	if value, ok := raw["is_archived"].(bool); ok && value {
		return true
	}

	return false
}

func exportDatabaseRows(c *client.Client, databaseID, dir string) error {
	rows, err := c.QueryAllDatabase(databaseID)
	if err != nil {
		return err
	}

	used := map[string]string{}

	for _, row := range rows {
		if isArchivedOrTrashed(row) {
			continue
		}

		rowID, ok := row["id"].(string)
		if !ok {
			continue
		}

		stem := writer.UniqueIDStem(rowID, used)
		writer.WriteMDStem(dir, stem, converter.RowToMarkdown(row, nil))

		blocks, err := c.GetAllBlocks(rowID)
		if err != nil {
			continue
		}

		md := converter.RowToMarkdown(row, blocks)
		writer.WriteMDStem(dir, stem, md)
	}

	return nil
}
