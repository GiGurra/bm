package fetcher

import (
	"strings"
	"testing"
)

func TestExtractText_Basic(t *testing.T) {
	html := `<html><body><h1>Hello World</h1><p>This is a paragraph.</p></body></html>`
	text := extractText(html)

	if !strings.Contains(text, "Hello World") {
		t.Errorf("expected 'Hello World' in output: %q", text)
	}
	if !strings.Contains(text, "This is a paragraph.") {
		t.Errorf("expected paragraph text in output: %q", text)
	}
}

func TestExtractText_StripsScriptAndStyle(t *testing.T) {
	html := `<html>
		<head><style>body { color: red; }</style></head>
		<body>
			<script>var x = 1; alert("hi");</script>
			<p>Visible content</p>
			<noscript>Enable JS</noscript>
		</body>
	</html>`
	text := extractText(html)

	if strings.Contains(text, "color: red") {
		t.Error("style content should be stripped")
	}
	if strings.Contains(text, "alert") {
		t.Error("script content should be stripped")
	}
	if strings.Contains(text, "Enable JS") {
		t.Error("noscript content should be stripped")
	}
	if !strings.Contains(text, "Visible content") {
		t.Errorf("visible content missing: %q", text)
	}
}

func TestExtractText_StripsNavHeaderFooter(t *testing.T) {
	html := `<html><body>
		<nav><a href="/">Home</a><a href="/about">About</a></nav>
		<header><h1>Site Header</h1></header>
		<main><p>Main content here</p></main>
		<footer>Copyright 2024</footer>
	</body></html>`
	text := extractText(html)

	if strings.Contains(text, "Home") {
		t.Error("nav content should be stripped")
	}
	if strings.Contains(text, "Site Header") {
		t.Error("header content should be stripped")
	}
	if strings.Contains(text, "Copyright") {
		t.Error("footer content should be stripped")
	}
	if !strings.Contains(text, "Main content here") {
		t.Errorf("main content missing: %q", text)
	}
}

func TestExtractText_BlockElements(t *testing.T) {
	html := `<html><body>
		<p>Paragraph one</p>
		<p>Paragraph two</p>
		<div>A div</div>
		<h2>A heading</h2>
		<ul><li>Item 1</li><li>Item 2</li></ul>
	</body></html>`
	text := extractText(html)

	// Block elements should produce newlines, not run together
	lines := strings.Split(text, "\n")
	if len(lines) < 3 {
		t.Errorf("expected multiple lines from block elements, got %d: %q", len(lines), text)
	}
}

func TestExtractText_WhitespaceCleanup(t *testing.T) {
	html := `<html><body>
		<p>   lots   of   spaces   </p>
		<p></p>
		<p>another line</p>
	</body></html>`
	text := extractText(html)

	if strings.Contains(text, "   ") {
		t.Errorf("excessive whitespace not cleaned: %q", text)
	}
	// Empty lines should be removed
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			t.Errorf("empty line found in output: %q", text)
			break
		}
	}
}

func TestExtractText_Empty(t *testing.T) {
	text := extractText("")
	if text != "" {
		t.Errorf("expected empty string, got %q", text)
	}
}

func TestExtractText_SizeCap(t *testing.T) {
	// Generate HTML with lots of content
	var sb strings.Builder
	sb.WriteString("<html><body>")
	for i := 0; i < 20000; i++ {
		sb.WriteString("<p>This is a line of text that should contribute to a very large output. </p>")
	}
	sb.WriteString("</body></html>")

	text := extractText(sb.String())
	if len(text) > 100_000 {
		t.Errorf("text exceeds 100K cap: %d chars", len(text))
	}
}

func TestExtractText_NestedElements(t *testing.T) {
	html := `<html><body>
		<div><span>Hello</span> <em>world</em></div>
	</body></html>`
	text := extractText(html)

	if !strings.Contains(text, "Hello") || !strings.Contains(text, "world") {
		t.Errorf("nested element text missing: %q", text)
	}
}
