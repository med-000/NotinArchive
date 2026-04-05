package service

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/med-000/notionarchive/notion/client"
	"github.com/med-000/notionarchive/notion/converter"
)

type IssueService struct {
	repo string
}

type issueRecord struct {
	Number int    `json:"number"`
	State  string `json:"state"`
}

func NewIssueServiceFromEnv() (*IssueService, error) {
	repo := strings.TrimSpace(os.Getenv("GITHUB_REPO"))
	if repo == "" {
		return nil, fmt.Errorf("GITHUB_REPO is required")
	}

	if _, err := exec.LookPath("gh"); err != nil {
		return nil, fmt.Errorf("gh command is required: %w", err)
	}

	return &IssueService{repo: repo}, nil
}

func (s *IssueService) UpsertIssue(notionID, title, body string, labels []string, closed bool) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("issue title is empty")
	}
	if strings.TrimSpace(notionID) == "" {
		return fmt.Errorf("notion id is empty")
	}

	labels = dedupeStrings(labels)
	if err := s.ensureLabels(labels); err != nil {
		return err
	}

	body = buildIssueBody(notionID, body)

	existing, err := s.findIssueByNotionID(notionID)
	if err != nil {
		return err
	}

	if existing != nil {
		return s.updateIssue(existing, title, body, labels, closed)
	}

	bodyFile, err := os.CreateTemp("", "notionarchive-issue-*.md")
	if err != nil {
		return err
	}
	defer os.Remove(bodyFile.Name())

	if _, err := bodyFile.WriteString(body); err != nil {
		bodyFile.Close()
		return err
	}
	if err := bodyFile.Close(); err != nil {
		return err
	}

	args := []string{"issue", "create", "--repo", s.repo, "--title", title, "--body-file", bodyFile.Name()}
	for _, label := range labels {
		args = append(args, "--label", label)
	}

	cmd := exec.Command("gh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh issue create failed: %s", strings.TrimSpace(string(out)))
	}

	issueRef := strings.TrimSpace(string(out))
	if closed && issueRef != "" {
		closeCmd := exec.Command("gh", "issue", "close", issueRef, "--repo", s.repo)
		closeOut, closeErr := closeCmd.CombinedOutput()
		if closeErr != nil {
			return fmt.Errorf("gh issue close failed: %s", strings.TrimSpace(string(closeOut)))
		}
	}

	return nil
}

func (s *IssueService) findIssueByNotionID(notionID string) (*issueRecord, error) {
	query := fmt.Sprintf("\"Notion-ID: %s\" in:body", notionID)

	cmd := exec.Command("gh", "issue", "list", "--repo", s.repo, "--state", "all", "--search", query, "--json", "number,state", "--limit", "10")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh issue list failed: %s", strings.TrimSpace(string(out)))
	}

	var issues []issueRecord
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, err
	}

	if len(issues) == 0 {
		return nil, nil
	}

	return &issues[0], nil
}

func (s *IssueService) updateIssue(issue *issueRecord, title, body string, labels []string, closed bool) error {
	bodyFile, err := os.CreateTemp("", "notionarchive-issue-*.md")
	if err != nil {
		return err
	}
	defer os.Remove(bodyFile.Name())

	if _, err := bodyFile.WriteString(body); err != nil {
		bodyFile.Close()
		return err
	}
	if err := bodyFile.Close(); err != nil {
		return err
	}

	number := strconv.Itoa(issue.Number)

	editCmd := exec.Command("gh", "issue", "edit", number, "--repo", s.repo, "--title", title, "--body-file", bodyFile.Name())
	editOut, editErr := editCmd.CombinedOutput()
	if editErr != nil {
		return fmt.Errorf("gh issue edit failed: %s", strings.TrimSpace(string(editOut)))
	}

	if err := s.syncLabels(number, labels); err != nil {
		return err
	}

	if closed && issue.State != "CLOSED" {
		closeCmd := exec.Command("gh", "issue", "close", number, "--repo", s.repo)
		closeOut, closeErr := closeCmd.CombinedOutput()
		if closeErr != nil {
			return fmt.Errorf("gh issue close failed: %s", strings.TrimSpace(string(closeOut)))
		}
	}

	if !closed && issue.State == "CLOSED" {
		reopenCmd := exec.Command("gh", "issue", "reopen", number, "--repo", s.repo)
		reopenOut, reopenErr := reopenCmd.CombinedOutput()
		if reopenErr != nil {
			return fmt.Errorf("gh issue reopen failed: %s", strings.TrimSpace(string(reopenOut)))
		}
	}

	return nil
}

func (s *IssueService) syncLabels(number string, desired []string) error {
	current, err := s.getIssueLabels(number)
	if err != nil {
		return err
	}

	desiredSet := toSet(desired)
	currentSet := toSet(current)

	for _, label := range desired {
		if currentSet[label] {
			continue
		}

		cmd := exec.Command("gh", "issue", "edit", number, "--repo", s.repo, "--add-label", label)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("gh issue add-label failed: %s", strings.TrimSpace(string(out)))
		}
	}

	for _, label := range current {
		if desiredSet[label] {
			continue
		}

		cmd := exec.Command("gh", "issue", "edit", number, "--repo", s.repo, "--remove-label", label)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("gh issue remove-label failed: %s", strings.TrimSpace(string(out)))
		}
	}

	return nil
}

func (s *IssueService) getIssueLabels(number string) ([]string, error) {
	cmd := exec.Command("gh", "issue", "view", number, "--repo", s.repo, "--json", "labels")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh issue view failed: %s", strings.TrimSpace(string(out)))
	}

	var payload struct {
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}

	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, err
	}

	labels := make([]string, 0, len(payload.Labels))
	for _, label := range payload.Labels {
		if strings.TrimSpace(label.Name) != "" {
			labels = append(labels, label.Name)
		}
	}

	return labels, nil
}

func (s *IssueService) ensureLabels(labels []string) error {
	for _, label := range labels {
		cmd := exec.Command("gh", "label", "create", label, "--repo", s.repo)
		out, err := cmd.CombinedOutput()
		if err != nil {
			message := strings.TrimSpace(string(out))
			if strings.Contains(message, "already exists") {
				continue
			}
			return fmt.Errorf("gh label create failed: %s", message)
		}
	}

	return nil
}

func SyncIssues() error {
	c := client.NewClient()
	issueService, err := NewIssueServiceFromEnv()
	if err != nil {
		return err
	}

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

		if objType == "page" {
			if !hasWorkspaceParent(raw) {
				continue
			}

			id, ok := raw["id"].(string)
			if !ok {
				continue
			}

			title := converter.ExtractTitle(raw)
			if err := walkIssues(c, issueService, id, title, map[string]bool{}); err != nil {
				return err
			}
		}

		if objType == "database" {
			if !hasWorkspaceParent(raw) {
				continue
			}

			id, ok := raw["id"].(string)
			if !ok {
				continue
			}

			if err := createIssuesFromDatabase(c, issueService, id); err != nil {
				if client.IsInaccessibleDatabaseError(err) {
					continue
				}
				return err
			}
		}
	}

	return nil
}

func walkIssues(c *client.Client, issueService *IssueService, pageID, pageTitle string, visited map[string]bool) error {
	if visited[pageID] {
		return nil
	}
	visited[pageID] = true

	rawBlocks, err := c.GetBlocks(pageID)
	if err != nil {
		return err
	}

	for _, raw := range rawBlocks {
		if isArchivedOrTrashed(raw) {
			continue
		}

		blockType, ok := raw["type"].(string)
		if !ok {
			continue
		}

		switch blockType {
		case "child_page":
			childID, ok := raw["id"].(string)
			if !ok {
				continue
			}

			childPage, ok := raw["child_page"].(map[string]interface{})
			if !ok {
				continue
			}

			childTitle, _ := childPage["title"].(string)
			if err := walkIssues(c, issueService, childID, childTitle, visited); err != nil {
				return err
			}
		case "child_database":
			dbID, ok := raw["id"].(string)
			if !ok {
				continue
			}

			if err := createIssuesFromDatabase(c, issueService, dbID); err != nil {
				if client.IsInaccessibleDatabaseError(err) {
					continue
				}
				return err
			}
		}
	}

	return nil
}

func createIssuesFromDatabase(c *client.Client, issueService *IssueService, databaseID string) error {
	rows, err := c.QueryAllDatabase(databaseID)
	if err != nil {
		return err
	}

	for _, row := range rows {
		if isArchivedOrTrashed(row) {
			continue
		}

		rowID, ok := row["id"].(string)
		if !ok {
			continue
		}

		properties := converter.ExtractProperties(row)
		title := converter.PropertyString(properties, "Title")
		if title == "" {
			title = converter.PropertyString(properties, "title")
		}
		if title == "" {
			title = converter.ExtractTitle(row)
		}

		status := converter.PropertyString(properties, "Status")
		labels := converter.PropertyStrings(properties, "Group")

		blocks, err := c.GetAllBlocks(rowID)
		if err != nil {
			blocks = nil
		}

		body := converter.BlocksBodyToMarkdown(blocks)
		if strings.TrimSpace(body) == "" {
			body = "(empty)\n"
		}

		if err := issueService.UpsertIssue(rowID, title, body, labels, converter.IsClosedStatus(status)); err != nil {
			return err
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

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}

	return result
}

func buildIssueBody(notionID, body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		body = "(empty)"
	}

	return fmt.Sprintf("Notion-ID: %s\n\n%s\n", notionID, body)
}

func toSet(values []string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result[value] = true
		}
	}
	return result
}
