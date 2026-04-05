package converter

import (
	"strings"
)

func BlocksToMarkdown(title string, blocks []map[string]interface{}) string {
	var md strings.Builder

	// 👇 タイトル埋め込み
	md.WriteString("# " + title + "\n\n")

	for _, b := range blocks {
		t, ok := b["type"].(string)
		if !ok {
			continue
		}

		switch t {

		case "paragraph":
			md.WriteString(extractText(b, "paragraph") + "\n\n")

		case "heading_1":
			md.WriteString("# " + extractText(b, "heading_1") + "\n\n")

		case "heading_2":
			md.WriteString("## " + extractText(b, "heading_2") + "\n\n")

		case "bulleted_list_item":
			md.WriteString("- " + extractText(b, "bulleted_list_item") + "\n")

		case "numbered_list_item":
			md.WriteString("1. " + extractText(b, "numbered_list_item") + "\n")

		case "to_do":
			md.WriteString("- [ ] " + extractText(b, "to_do") + "\n")

		case "code":
			md.WriteString("```\n" + extractText(b, "code") + "\n```\n\n")
		}
	}

	return md.String()
}

func extractText(block map[string]interface{}, key string) string {
	obj, ok := block[key].(map[string]interface{})
	if !ok {
		return ""
	}

	richTexts, ok := obj["rich_text"].([]interface{})
	if !ok {
		return ""
	}

	var result strings.Builder

	for _, rt := range richTexts {
		m := rt.(map[string]interface{})
		text := m["plain_text"].(string)
		result.WriteString(text)
	}

	return result.String()
}

func ExtractTitle(raw map[string]interface{}) string {
	props, ok := raw["properties"].(map[string]interface{})
	if !ok {
		return "untitled"
	}

	for _, v := range props {
		prop := v.(map[string]interface{})

		if prop["type"] == "title" {
			arr := prop["title"].([]interface{})
			if len(arr) == 0 {
				return "untitled"
			}

			return arr[0].(map[string]interface{})["plain_text"].(string)
		}
	}

	return "untitled"
}
