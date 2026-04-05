package converter

import (
	"fmt"
	"sort"
	"strconv"
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

func RowToMarkdown(raw map[string]interface{}, blocks []map[string]interface{}) string {
	title := ExtractTitle(raw)
	properties := ExtractProperties(raw)

	var md strings.Builder
	md.WriteString(BuildFrontMatter(raw, properties))
	md.WriteString(BlocksToMarkdown(title, blocks))

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

func ExtractProperties(raw map[string]interface{}) map[string]interface{} {
	props, ok := raw["properties"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{}
	}

	result := map[string]interface{}{}

	for name, value := range props {
		prop, ok := value.(map[string]interface{})
		if !ok {
			continue
		}

		propType, ok := prop["type"].(string)
		if !ok {
			continue
		}

		normalized, ok := normalizeProperty(propType, prop)
		if !ok {
			continue
		}

		result[name] = normalized
	}

	return result
}

func BuildFrontMatter(raw map[string]interface{}, properties map[string]interface{}) string {
	var md strings.Builder

	md.WriteString("---\n")

	if id, ok := raw["id"].(string); ok && id != "" {
		md.WriteString("notion_id: " + yamlString(id) + "\n")
	}

	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	md.WriteString("properties:\n")
	for _, key := range keys {
		writeYAMLField(&md, 1, key, properties[key])
	}

	md.WriteString("---\n\n")

	return md.String()
}

func normalizeProperty(propType string, prop map[string]interface{}) (interface{}, bool) {
	switch propType {
	case "title":
		return extractPlainTexts(prop["title"]), true
	case "rich_text":
		return extractPlainTexts(prop["rich_text"]), true
	case "number":
		if value, ok := prop["number"]; ok && value != nil {
			return value, true
		}
		return nil, false
	case "select":
		if value, ok := prop["select"].(map[string]interface{}); ok {
			if name, ok := value["name"].(string); ok && name != "" {
				return name, true
			}
		}
		return nil, false
	case "multi_select":
		if values, ok := prop["multi_select"].([]interface{}); ok {
			result := make([]string, 0, len(values))
			for _, value := range values {
				item, ok := value.(map[string]interface{})
				if !ok {
					continue
				}
				name, ok := item["name"].(string)
				if ok && name != "" {
					result = append(result, name)
				}
			}
			return result, len(result) > 0
		}
		return nil, false
	case "status":
		if value, ok := prop["status"].(map[string]interface{}); ok {
			if name, ok := value["name"].(string); ok && name != "" {
				return name, true
			}
		}
		return nil, false
	case "date":
		if value, ok := prop["date"].(map[string]interface{}); ok {
			result := map[string]interface{}{}
			if start, ok := value["start"].(string); ok && start != "" {
				result["start"] = start
			}
			if end, ok := value["end"].(string); ok && end != "" {
				result["end"] = end
			}
			if tz, ok := value["time_zone"].(string); ok && tz != "" {
				result["time_zone"] = tz
			}
			return result, len(result) > 0
		}
		return nil, false
	case "checkbox":
		if value, ok := prop["checkbox"].(bool); ok {
			return value, true
		}
		return nil, false
	case "url":
		if value, ok := prop["url"].(string); ok && value != "" {
			return value, true
		}
		return nil, false
	case "email":
		if value, ok := prop["email"].(string); ok && value != "" {
			return value, true
		}
		return nil, false
	case "phone_number":
		if value, ok := prop["phone_number"].(string); ok && value != "" {
			return value, true
		}
		return nil, false
	case "created_time":
		if value, ok := prop["created_time"].(string); ok && value != "" {
			return value, true
		}
		return nil, false
	case "last_edited_time":
		if value, ok := prop["last_edited_time"].(string); ok && value != "" {
			return value, true
		}
		return nil, false
	case "relation":
		if values, ok := prop["relation"].([]interface{}); ok {
			result := make([]string, 0, len(values))
			for _, value := range values {
				item, ok := value.(map[string]interface{})
				if !ok {
					continue
				}
				id, ok := item["id"].(string)
				if ok && id != "" {
					result = append(result, id)
				}
			}
			return result, len(result) > 0
		}
		return nil, false
	case "people":
		if values, ok := prop["people"].([]interface{}); ok {
			result := make([]string, 0, len(values))
			for _, value := range values {
				item, ok := value.(map[string]interface{})
				if !ok {
					continue
				}
				if name, ok := item["name"].(string); ok && name != "" {
					result = append(result, name)
					continue
				}
				if id, ok := item["id"].(string); ok && id != "" {
					result = append(result, id)
				}
			}
			return result, len(result) > 0
		}
		return nil, false
	case "formula":
		if value, ok := prop["formula"].(map[string]interface{}); ok {
			formulaType, _ := value["type"].(string)
			switch formulaType {
			case "string":
				if v, ok := value["string"].(string); ok && v != "" {
					return v, true
				}
			case "number":
				if v, ok := value["number"]; ok && v != nil {
					return v, true
				}
			case "boolean":
				if v, ok := value["boolean"].(bool); ok {
					return v, true
				}
			case "date":
				if v, ok := value["date"].(map[string]interface{}); ok {
					return normalizeProperty("date", map[string]interface{}{"date": v})
				}
			}
		}
		return nil, false
	default:
		return nil, false
	}
}

func extractPlainTexts(raw interface{}) string {
	values, ok := raw.([]interface{})
	if !ok {
		return ""
	}

	var result strings.Builder

	for _, value := range values {
		item, ok := value.(map[string]interface{})
		if !ok {
			continue
		}

		text, ok := item["plain_text"].(string)
		if ok {
			result.WriteString(text)
		}
	}

	return result.String()
}

func writeYAMLField(md *strings.Builder, indent int, key string, value interface{}) {
	prefix := strings.Repeat("  ", indent)
	quotedKey := yamlKey(key)

	switch v := value.(type) {
	case string:
		md.WriteString(fmt.Sprintf("%s%s: %s\n", prefix, quotedKey, yamlString(v)))
	case bool:
		md.WriteString(fmt.Sprintf("%s%s: %t\n", prefix, quotedKey, v))
	case float64:
		md.WriteString(fmt.Sprintf("%s%s: %v\n", prefix, quotedKey, v))
	case int:
		md.WriteString(fmt.Sprintf("%s%s: %d\n", prefix, quotedKey, v))
	case []string:
		if len(v) == 0 {
			md.WriteString(fmt.Sprintf("%s%s: []\n", prefix, quotedKey))
			return
		}
		md.WriteString(fmt.Sprintf("%s%s:\n", prefix, quotedKey))
		for _, item := range v {
			md.WriteString(fmt.Sprintf("%s  - %s\n", prefix, yamlString(item)))
		}
	case map[string]interface{}:
		if len(v) == 0 {
			md.WriteString(fmt.Sprintf("%s%s: {}\n", prefix, quotedKey))
			return
		}
		md.WriteString(fmt.Sprintf("%s%s:\n", prefix, quotedKey))
		keys := make([]string, 0, len(v))
		for nestedKey := range v {
			keys = append(keys, nestedKey)
		}
		sort.Strings(keys)
		for _, nestedKey := range keys {
			writeYAMLField(md, indent+1, nestedKey, v[nestedKey])
		}
	default:
		md.WriteString(fmt.Sprintf("%s%s: %s\n", prefix, quotedKey, yamlString(fmt.Sprint(v))))
	}
}

func yamlKey(s string) string {
	return strconv.Quote(s)
}

func yamlString(s string) string {
	return strconv.Quote(s)
}
