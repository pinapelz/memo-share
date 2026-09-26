package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

func getLinkInformation(link string) (title string, favicon string, err error) {
	parsedURL, err := url.ParseRequestURI(link)
	if err != nil {
		return "", "", fmt.Errorf("invalid url: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return "", "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("parse html: %w", err)
	}

	title = extractTitle(doc)
	favicon = extractFavicon(doc, parsedURL)

	if favicon == "" {
		favicon = parsedURL.ResolveReference(&url.URL{Path: "/favicon.ico"}).String()
	}

	return title, favicon, nil
}

func extractTitle(n *html.Node) string {
	var walk func(*html.Node) string
	walk = func(node *html.Node) string {
		if node.Type == html.ElementNode && node.Data == "title" {
			if node.FirstChild != nil {
				return strings.TrimSpace(node.FirstChild.Data)
			}
		}

		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if result := walk(c); result != "" {
				return result
			}
		}
		return ""
	}

	return walk(n)
}

func extractFavicon(n *html.Node, base *url.URL) string {
	var faviconHref string
	var shortcutHref string

	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "link" {
			rel := ""
			href := ""
			for _, attr := range node.Attr {
				switch strings.ToLower(attr.Key) {
				case "rel":
					rel = strings.ToLower(strings.TrimSpace(attr.Val))
				case "href":
					href = strings.TrimSpace(attr.Val)
				}
			}

			if href != "" {
				if strings.Contains(rel, "icon") {
					if strings.Contains(rel, "shortcut") && shortcutHref == "" {
						shortcutHref = href
					}
					if faviconHref == "" {
						faviconHref = href
					}
				}
			}
		}

		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}

	walk(n)

	candidate := faviconHref
	if candidate == "" {
		candidate = shortcutHref
	}
	if candidate == "" {
		return ""
	}

	iconURL, err := url.Parse(candidate)
	if err != nil {
		return ""
	}

	return base.ResolveReference(iconURL).String()
}
