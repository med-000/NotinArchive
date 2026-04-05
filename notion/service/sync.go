package service

import (
	"fmt"
	"path/filepath"

	"github.com/med-000/notionarchive/notion/client"
	"github.com/med-000/notionarchive/notion/converter"
	"github.com/med-000/notionarchive/notion/walker"
	"github.com/med-000/notionarchive/notion/writer"
)

func Sync(rootDir string) error {
	c := client.NewClient()

	results, err := c.Search()
	if err != nil {
		return err
	}

	for _, raw := range results {
		if isArchivedOrTrashed(raw) {
			continue
		}

		objType, ok := raw["object"].(string)
		if !ok {
			continue
		}

		// -------- page --------
		if objType == "page" {
			if !hasWorkspaceParent(raw) {
				continue
			}

			id, ok := raw["id"].(string)
			if !ok {
				continue
			}

			title := converter.ExtractTitle(raw)
			dir := filepath.Join(rootDir, writer.Sanitize(title))

			fmt.Println("page:", dir)

			writer.CreateDir(dir)
			writer.WriteMD(dir, id, title)

			walker.Walk(c, id, dir, title, map[string]bool{})
		}

		// -------- database --------
		if objType == "database" {
			if !hasWorkspaceParent(raw) {
				continue
			}

			id, ok := raw["id"].(string)
			if !ok {
				continue
			}

			title := extractDatabaseTitle(raw)
			dir := filepath.Join(rootDir, writer.Sanitize(title))
			writer.CreateDir(dir)

			if err := exportDatabaseRows(c, id, dir); err != nil {
				if client.IsInaccessibleDatabaseError(err) {
					fmt.Println("skip inaccessible db:", id)
					continue
				}
				continue
			}
		}
	}

	return nil
}

func hasWorkspaceParent(raw map[string]interface{}) bool {
	parent, ok := raw["parent"].(map[string]interface{})
	if !ok {
		return false
	}

	parentType, ok := parent["type"].(string)
	return ok && parentType == "workspace"
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

func extractDatabaseTitle(raw map[string]interface{}) string {
	title, ok := raw["title"].([]interface{})
	if !ok || len(title) == 0 {
		return "untitled"
	}

	first, ok := title[0].(map[string]interface{})
	if !ok {
		return "untitled"
	}

	plainText, ok := first["plain_text"].(string)
	if !ok || plainText == "" {
		return "untitled"
	}

	return plainText
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
