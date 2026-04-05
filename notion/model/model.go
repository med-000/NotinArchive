package model

type Page struct {
	ID     string `json:"id"`
	Object string `json:"object"`

	Properties struct {
		Title struct {
			Title []struct {
				PlainText string `json:"plain_text"`
			} `json:"title"`
		} `json:"title"`
	} `json:"properties"`
}

type Block struct {
	ID   string `json:"id"`
	Type string `json:"type"`

	ChildPage struct {
		Title string `json:"title"`
	} `json:"child_page"`

	ChildDatabase struct {
		Title string `json:"title"`
	} `json:"child_database"`
}
