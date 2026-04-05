package converter

import "strings"

func BlocksBodyToMarkdown(blocks []map[string]interface{}) string {
	var md strings.Builder

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

	return strings.TrimSpace(md.String()) + "\n"
}

func PropertyString(properties map[string]interface{}, key string) string {
	value, ok := properties[key]
	if !ok {
		return ""
	}

	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return ""
	}
}

func PropertyStrings(properties map[string]interface{}, key string) []string {
	value, ok := properties[key]
	if !ok {
		return nil
	}

	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{strings.TrimSpace(v)}
	case []string:
		result := make([]string, 0, len(v))
		for _, item := range v {
			item = strings.TrimSpace(item)
			if item != "" {
				result = append(result, item)
			}
		}
		return result
	default:
		return nil
	}
}

func IsClosedStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "done", "closed", "complete", "completed", "完了", "完了済み", "close":
		return true
	default:
		return false
	}
}
