package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const defaultProxy = "http://127.0.0.1:10808"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "fetch":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: proxy-fetch fetch <url> [--raw]")
			os.Exit(1)
		}
		targetURL := os.Args[2]
		raw := len(os.Args) > 3 && os.Args[3] == "--raw"
		doFetch(targetURL, raw)

	case "test":
		targetURL := "https://httpbin.org/ip"
		if len(os.Args) > 2 {
			targetURL = os.Args[2]
		}
		doTest(targetURL)

	case "api":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: proxy-fetch api <url> [--method POST] [--body '{...}'] [--header 'K: V']")
			os.Exit(1)
		}
		doAPI(os.Args[2:])

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`proxy-fetch — 通过代理抓取网页和 API

Commands:
  fetch <url> [--raw]     抓取网页并提取正文（--raw 输出原始 HTML）
  test  [url]             测试代理连通性
  api   <url> [options]   发送 API 请求（支持自定义方法/body/header）

API Options:
  --method METHOD         HTTP 方法（默认 GET）
  --body   BODY           请求体（JSON 字符串）
  --header "Key: Value"   自定义请求头（可多次指定）

Environment:
  HTTPS_PROXY / HTTP_PROXY   代理地址（默认 ` + defaultProxy + `）`)
}

func getProxyClient() *http.Client {
	proxyAddr := os.Getenv("HTTPS_PROXY")
	if proxyAddr == "" {
		proxyAddr = os.Getenv("HTTP_PROXY")
	}
	if proxyAddr == "" {
		proxyAddr = defaultProxy
	}

	proxyURL, err := url.Parse(proxyAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid proxy URL: %v\n", err)
		os.Exit(1)
	}

	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
		Timeout: 30 * time.Second,
	}
}

// === fetch ===

func doFetch(targetURL string, raw bool) {
	client := getProxyClient()

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; proxy-fetch/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5,zh-CN;q=0.3")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fetch failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Read body failed: %v\n", err)
		os.Exit(1)
	}

	// 输出元信息
	fmt.Fprintf(os.Stderr, "[%d] %s (%d bytes)\n", resp.StatusCode, resp.Status, len(body))

	if raw {
		fmt.Print(string(body))
		return
	}

	// 判断内容类型
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "json") {
		// JSON：格式化输出
		var obj interface{}
		if err := json.Unmarshal(body, &obj); err == nil {
			pretty, _ := json.MarshalIndent(obj, "", "  ")
			fmt.Println(string(pretty))
		} else {
			fmt.Print(string(body))
		}
		return
	}

	if strings.Contains(ct, "text/plain") {
		fmt.Print(string(body))
		return
	}

	// HTML：提取正文
	text := extractReadableText(string(body))
	fmt.Print(text)
}

// === test ===

func doTest(targetURL string) {
	client := getProxyClient()

	start := time.Now()
	resp, err := client.Get(targetURL)
	elapsed := time.Since(start)

	if err != nil {
		fmt.Printf("FAIL: %v (%.0fms)\n", err, float64(elapsed.Milliseconds()))
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	fmt.Printf("OK: %d %s (%.0fms)\n", resp.StatusCode, resp.Status, float64(elapsed.Milliseconds()))
	if len(body) > 0 && len(body) < 1024 {
		fmt.Println(string(body))
	}
}

// === api ===

func doAPI(args []string) {
	targetURL := args[0]
	method := "GET"
	var bodyStr string
	var headers []string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--method":
			if i+1 < len(args) {
				method = args[i+1]
				i++
			}
		case "--body":
			if i+1 < len(args) {
				bodyStr = args[i+1]
				i++
			}
		case "--header":
			if i+1 < len(args) {
				headers = append(headers, args[i+1])
				i++
			}
		}
	}

	client := getProxyClient()

	var bodyReader io.Reader
	if bodyStr != "" {
		bodyReader = strings.NewReader(bodyStr)
	}

	req, err := http.NewRequest(method, targetURL, bodyReader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
		os.Exit(1)
	}

	req.Header.Set("User-Agent", "proxy-fetch/1.0")
	if bodyStr != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	for _, h := range headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	fmt.Fprintf(os.Stderr, "[%d] %s\n", resp.StatusCode, resp.Status)

	// 尝试格式化 JSON
	var obj interface{}
	if err := json.Unmarshal(respBody, &obj); err == nil {
		pretty, _ := json.MarshalIndent(obj, "", "  ")
		fmt.Println(string(pretty))
	} else {
		fmt.Print(string(respBody))
	}
}

// === HTML 内容提取 ===

func extractReadableText(htmlStr string) string {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return stripTags(htmlStr)
	}

	// 提取 title
	title := extractTitle(doc)

	// 移除 script, style, nav, footer, header, aside
	removeElements(doc, map[string]bool{
		"script": true, "style": true, "noscript": true,
		"nav": true, "footer": true, "aside": true,
		"iframe": true, "svg": true,
	})

	// 提取文本
	var sb strings.Builder
	if title != "" {
		sb.WriteString("# " + title + "\n\n")
	}
	extractText(doc, &sb, 0)

	text := sb.String()

	// 清理多余空行
	reBlankLines := regexp.MustCompile(`\n{3,}`)
	text = reBlankLines.ReplaceAllString(text, "\n\n")
	text = strings.TrimSpace(text)

	return text
}

func extractTitle(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "title" {
		return getTextContent(n)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if t := extractTitle(c); t != "" {
			return t
		}
	}
	return ""
}

func getTextContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(getTextContent(c))
	}
	return sb.String()
}

func removeElements(n *html.Node, tags map[string]bool) {
	var toRemove []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && tags[c.Data] {
			toRemove = append(toRemove, c)
		} else {
			removeElements(c, tags)
		}
	}
	for _, c := range toRemove {
		n.RemoveChild(c)
	}
}

func extractText(n *html.Node, sb *strings.Builder, depth int) {
	if n.Type == html.TextNode {
		text := strings.TrimSpace(n.Data)
		if text != "" {
			sb.WriteString(text)
			sb.WriteString(" ")
		}
		return
	}

	if n.Type == html.ElementNode {
		switch n.Data {
		case "h1":
			sb.WriteString("\n\n## ")
		case "h2":
			sb.WriteString("\n\n### ")
		case "h3", "h4", "h5", "h6":
			sb.WriteString("\n\n#### ")
		case "p", "div", "section", "article", "main":
			sb.WriteString("\n")
		case "br":
			sb.WriteString("\n")
		case "li":
			sb.WriteString("\n- ")
		case "a":
			// 提取链接
			href := getAttr(n, "href")
			linkText := getTextContent(n)
			linkText = strings.TrimSpace(linkText)
			if linkText != "" && href != "" && !strings.HasPrefix(href, "javascript:") {
				sb.WriteString(fmt.Sprintf("[%s](%s) ", linkText, href))
				return // 不再递归子节点
			}
		case "code", "pre":
			sb.WriteString("\n```\n")
			sb.WriteString(getTextContent(n))
			sb.WriteString("\n```\n")
			return
		case "img":
			alt := getAttr(n, "alt")
			if alt != "" {
				sb.WriteString(fmt.Sprintf("[image: %s] ", alt))
			}
			return
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractText(c, sb, depth+1)
	}

	if n.Type == html.ElementNode {
		switch n.Data {
		case "h1", "h2", "h3", "h4", "h5", "h6":
			sb.WriteString("\n")
		case "p":
			sb.WriteString("\n")
		}
	}
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func stripTags(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(s, "")
}
