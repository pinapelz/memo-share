package main

type Entry struct {
	ID       string
	Content  string
	Type     string
	Filename string
	Icon     *string
}

type linkRecord struct {
	Content string `json:"content"`
	Title   string `json:"title"`
	Favicon string `json:"favicon,omitempty"`
}
