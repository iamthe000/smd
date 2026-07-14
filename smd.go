package main

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

type Attributes map[string]any

type ASTNode struct {
	Type       string     `json:"type"`
	Attributes Attributes `json:"attributes,omitempty"`
	Children   []*ASTNode `json:"children,omitempty"`
	Value      string     `json:"value,omitempty"`
	Level      int        `json:"level,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type TokenType int

const (
	TokenText TokenType = iota
	TokenNewline
	TokenEOF
)

type Token struct {
	Type  TokenType
	Value string
}

type Lexer struct {
	src []byte
	pos int
}

func NewLexer(src string) *Lexer { return &Lexer{src: []byte(src)} }

func (l *Lexer) NextToken() Token {
	if l.pos >= len(l.src) {
		return Token{Type: TokenEOF}
	}
	if l.src[l.pos] == '\n' {
		l.pos++
		return Token{Type: TokenNewline, Value: "\n"}
	}
	start := l.pos
	for l.pos < len(l.src) && l.src[l.pos] != '\n' {
		if l.src[l.pos] == '\\' && l.pos+1 < len(l.src) {
			l.pos += 2
			continue
		}
		l.pos++
	}
	return Token{Type: TokenText, Value: string(l.src[start:l.pos])}
}

func Parse(src string) (*ASTNode, error) {
	body, definitions, err := extractDefinitions(src)
	if err != nil {
		return nil, err
	}
	p := &parser{
		lines: splitLines(body),
	}
	children, err := p.parseBlocks(0)
	if err != nil {
		return nil, err
	}
	children = append(children, definitions...)
	document := &ASTNode{Type: "Document", Children: children}
	analyzeDocument(document)
	return document, nil
}

func RenderHTML(node *ASTNode) (string, error) {
	var b strings.Builder
	if err := renderHTML(&b, node); err != nil {
		return "", err
	}
	return b.String(), nil
}

func renderHTML(b *strings.Builder, n *ASTNode) error {
	switch n.Type {
	case "Document":
		for _, c := range n.Children {
			if err := renderHTML(b, c); err != nil {
				return err
			}
		}
		renderEndnotes(b, n)
		renderBibliography(b, n)
	case "Heading":
		b.WriteString(fmt.Sprintf("<h%d", n.Level))
		writeHTMLAttrs(b, filteredAttrs(n.Attributes, "number"))
		b.WriteString(">")
		if number, ok := n.Attributes["number"]; ok {
			b.WriteString(`<span class="heading-number">`)
			b.WriteString(html.EscapeString(fmt.Sprint(number)))
			b.WriteString(`</span> `)
		}
		for _, c := range n.Children {
			if err := renderHTML(b, c); err != nil {
				return err
			}
		}
		b.WriteString(fmt.Sprintf("</h%d>", n.Level))
	case "Paragraph":
		b.WriteString("<p")
		writeHTMLAttrs(b, n.Attributes)
		b.WriteString(">")
		for _, c := range n.Children {
			if err := renderHTML(b, c); err != nil {
				return err
			}
		}
		b.WriteString("</p>")
	case "Text":
		b.WriteString(html.EscapeString(n.Value))
	case "Bold":
		b.WriteString("<strong>")
		for _, c := range n.Children {
			if err := renderHTML(b, c); err != nil {
				return err
			}
		}
		b.WriteString("</strong>")
	case "Italic":
		b.WriteString("<em>")
		for _, c := range n.Children {
			if err := renderHTML(b, c); err != nil {
				return err
			}
		}
		b.WriteString("</em>")
	case "Code":
		b.WriteString("<code>")
		b.WriteString(html.EscapeString(n.Value))
		b.WriteString("</code>")
	case "Link":
		b.WriteString("<a")
		writeHTMLAttrs(b, n.Attributes)
		b.WriteString(">")
		for _, c := range n.Children {
			if err := renderHTML(b, c); err != nil {
				return err
			}
		}
		b.WriteString("</a>")
	case "TextSpan":
		b.WriteString("<span")
		writeHTMLAttrs(b, n.Attributes)
		b.WriteString(">")
		for _, c := range n.Children {
			if err := renderHTML(b, c); err != nil {
				return err
			}
		}
		b.WriteString("</span>")
	case "FootnoteRef":
		key := fmt.Sprint(n.Attributes["key"])
		b.WriteString(`<sup class="footnote-ref" id="fnref-`)
		b.WriteString(html.EscapeString(key))
		b.WriteString(`"><a href="#fn-`)
		b.WriteString(html.EscapeString(key))
		b.WriteString(`">`)
		b.WriteString(fmt.Sprint(n.Attributes["number"]))
		b.WriteString(`</a></sup>`)
	case "Citation":
		key := fmt.Sprint(n.Attributes["key"])
		b.WriteString(`<a class="citation" href="#ref-`)
		b.WriteString(html.EscapeString(key))
		b.WriteString(`">[`)
		b.WriteString(fmt.Sprint(n.Attributes["number"]))
		b.WriteString(`]</a>`)
	case "CrossReference":
		b.WriteString(`<a class="cross-reference" href="#`)
		b.WriteString(html.EscapeString(fmt.Sprint(n.Attributes["target"])))
		b.WriteString(`">`)
		b.WriteString(html.EscapeString(fmt.Sprint(n.Attributes["label"])))
		b.WriteString(` `)
		b.WriteString(html.EscapeString(fmt.Sprint(n.Attributes["number"])))
		b.WriteString(`</a>`)
	case "FootnoteDefinition", "BibliographyEntry":
		// Definitions are rendered in their dedicated document sections.
	case "TableOfContents":
		b.WriteString(`<nav class="table-of-contents" aria-label="Table of contents"><h2>目次</h2><ol>`)
		for _, heading := range n.Children {
			b.WriteString(`<li class="toc-level-`)
			b.WriteString(strconv.Itoa(heading.Level))
			b.WriteString(`"><a href="#`)
			b.WriteString(html.EscapeString(fmt.Sprint(heading.Attributes["id"])))
			b.WriteString(`">`)
			b.WriteString(html.EscapeString(fmt.Sprint(heading.Attributes["number"])))
			b.WriteString(` `)
			b.WriteString(html.EscapeString(plainText(heading)))
			b.WriteString(`</a></li>`)
		}
		b.WriteString(`</ol></nav>`)
	case "Directive":
		if n.Name == "figure" {
			renderFigure(b, n)
			return nil
		}
		b.WriteString("<div")
		writeHTMLAttrs(b, withClass(n.Attributes, "directive"))
		b.WriteString(" data-directive=\"")
		b.WriteString(html.EscapeString(n.Name))
		b.WriteString("\">")
		for _, c := range n.Children {
			if err := renderHTML(b, c); err != nil {
				return err
			}
		}
		b.WriteString("</div>")
	case "Component":
		b.WriteString("<div")
		writeHTMLAttrs(b, withClass(n.Attributes, "component-"+strings.ToLower(n.Name)))
		b.WriteString(" data-component=\"")
		b.WriteString(html.EscapeString(n.Name))
		b.WriteString("\">")
		for _, c := range n.Children {
			if err := renderHTML(b, c); err != nil {
				return err
			}
		}
		b.WriteString("</div>")
	default:
		for _, c := range n.Children {
			if err := renderHTML(b, c); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeHTMLAttrs(b *strings.Builder, attrs Attributes) {
	if len(attrs) == 0 {
		return
	}
	for _, key := range sortedKeys(attrs) {
		val := attrs[key]
		switch v := val.(type) {
		case bool:
			if v {
				b.WriteString(" ")
				b.WriteString(key)
			}
		default:
			b.WriteString(" ")
			b.WriteString(key)
			b.WriteString("=\"")
			b.WriteString(html.EscapeString(fmt.Sprint(v)))
			b.WriteString("\"")
		}
	}
}

func withClass(attrs Attributes, prefix string) Attributes {
	out := make(Attributes, len(attrs)+1)
	for key, value := range attrs {
		out[key] = value
	}
	if class, ok := out["class"]; ok && fmt.Sprint(class) != "" {
		out["class"] = prefix + " " + fmt.Sprint(class)
	} else {
		out["class"] = prefix
	}
	return out
}

func addClass(b *strings.Builder, class string) {
	b.WriteString(" class=\"")
	b.WriteString(html.EscapeString(class))
	b.WriteString("\"")
}

func addClassFromAttr(b *strings.Builder, attrs Attributes) {
	if attrs == nil {
		return
	}
	if cls, ok := attrs["class"]; ok {
		b.WriteString(" class=\"")
		b.WriteString(html.EscapeString(fmt.Sprint(cls)))
		b.WriteString("\"")
	}
}

func filteredAttrs(attrs Attributes, exclude string) Attributes {
	if len(attrs) == 0 {
		return nil
	}
	out := make(Attributes, len(attrs))
	for k, v := range attrs {
		if k != exclude {
			out[k] = v
		}
	}
	return out
}

type parser struct {
	lines []string
	i     int
}

func (p *parser) parseBlocks(indent int) ([]*ASTNode, error) {
	var nodes []*ASTNode
	for p.i < len(p.lines) {
		line := p.lines[p.i]
		if strings.TrimSpace(line) == "" {
			p.i++
			continue
		}
		curIndent := countIndent(line)
		if curIndent < indent {
			break
		}
		trimmed := strings.TrimSpace(line)
		// Container parsers consume their own closing delimiter. Leave it for
		// the caller instead of interpreting it as another container opener.
		if trimmed == ":::" || strings.HasPrefix(trimmed, "</@") {
			break
		}
		if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") && isBlockAttrLine(trimmed) {
			attrs, err := parseAttributeBlock(trimmed)
			if err != nil {
				return nil, err
			}
			p.i++
			node, err := p.parseNextBlock(indent)
			if err != nil {
				return nil, err
			}
			if node == nil {
				continue
			}
			mergeAttributes(node, attrs)
			nodes = append(nodes, node)
			continue
		}
		node, err := p.parseNextBlock(indent)
		if err != nil {
			return nil, err
		}
		if node != nil {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func (p *parser) parseNextBlock(indent int) (*ASTNode, error) {
	if p.i >= len(p.lines) {
		return nil, nil
	}
	line := p.lines[p.i]
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, ":::") {
		return p.parseDirective(indent)
	}
	if strings.HasPrefix(trimmed, "<@") {
		return p.parseComponent(indent)
	}
	if isHeadingLine(trimmed) {
		p.i++
		level := countPrefix(trimmed, '#')
		content := strings.TrimSpace(trimmed[level:])
		content, attrs := extractTrailingAttributes(content)
		inlines, err := parseInlines(content)
		if err != nil {
			return nil, err
		}
		return &ASTNode{Type: "Heading", Level: level, Attributes: attrs, Children: inlines}, nil
	}
	return p.parseParagraph(indent)
}

func (p *parser) parseParagraph(indent int) (*ASTNode, error) {
	var buf []string
	attrs := Attributes{}
	for p.i < len(p.lines) {
		line := p.lines[p.i]
		if strings.TrimSpace(line) == "" {
			p.i++
			break
		}
		if countIndent(line) < indent {
			break
		}
		trimmed := strings.TrimSpace(line)
		if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") && isBlockAttrLine(trimmed)) ||
			strings.HasPrefix(trimmed, ":::") || strings.HasPrefix(trimmed, "<@") || isHeadingLine(trimmed) {
			break
		}
		buf = append(buf, strings.TrimSpace(line))
		p.i++
	}
	if len(buf) == 0 {
		return nil, nil
	}
	text := strings.Join(buf, " ")
	text, attrs = extractTrailingAttributes(text)
	inlines, err := parseInlines(text)
	if err != nil {
		return nil, err
	}
	return &ASTNode{Type: "Paragraph", Attributes: attrs, Children: inlines}, nil
}

func (p *parser) parseDirective(indent int) (*ASTNode, error) {
	header := strings.TrimSpace(p.lines[p.i])
	if !strings.HasPrefix(header, ":::") {
		return nil, fmt.Errorf("invalid directive")
	}
	rest := strings.TrimSpace(strings.TrimPrefix(header, ":::"))
	p.i++
	parts := splitFields(rest)
	if len(parts) == 0 {
		return nil, fmt.Errorf("directive requires name")
	}
	name := parts[0]
	attrs, err := parseAttributeFields(parts[1:])
	if err != nil {
		return nil, err
	}
	// Directive contents may be indented for readability, but indentation is
	// optional (as in the canonical alert example).
	children, err := p.parseBlocks(indent)
	if err != nil {
		return nil, err
	}
	if p.i >= len(p.lines) || strings.TrimSpace(p.lines[p.i]) != ":::" {
		return nil, fmt.Errorf("unterminated directive %s", name)
	}
	p.i++
	return &ASTNode{Type: "Directive", Name: name, Attributes: attrs, Children: children}, nil
}

func (p *parser) parseComponent(indent int) (*ASTNode, error) {
	header := strings.TrimSpace(p.lines[p.i])
	if !strings.HasPrefix(header, "<@") {
		return nil, fmt.Errorf("invalid component")
	}
	selfClosing := strings.HasSuffix(header, "/>")
	body := strings.TrimSpace(strings.TrimPrefix(header, "<@"))
	body = strings.TrimSuffix(body, ">")
	body = strings.TrimSuffix(body, "/")
	name, attrs, err := parseComponentHeader(body)
	if err != nil {
		return nil, err
	}
	p.i++
	if selfClosing {
		return &ASTNode{Type: "Component", Name: name, Attributes: attrs}, nil
	}
	children, err := p.parseBlocks(indent)
	if err != nil {
		return nil, err
	}
	if p.i >= len(p.lines) || strings.TrimSpace(p.lines[p.i]) != fmt.Sprintf("</@%s>", name) {
		return nil, fmt.Errorf("unterminated component %s", name)
	}
	p.i++
	return &ASTNode{Type: "Component", Name: name, Attributes: attrs, Children: children}, nil
}

func parseComponentHeader(s string) (string, Attributes, error) {
	fields := splitFields(s)
	if len(fields) == 0 {
		return "", nil, fmt.Errorf("missing component name")
	}
	name := strings.TrimPrefix(fields[0], "@")
	attrs, err := parseAttributeFields(fields[1:])
	return name, attrs, err
}

func parseInlines(s string) ([]*ASTNode, error) {
	var out []*ASTNode
	for len(s) > 0 {
		switch {
		case strings.HasPrefix(s, "**"):
			inner, rest, ok := takeDelimited(s[2:], "**")
			if ok {
				nodes, err := parseInlines(inner)
				if err != nil {
					return nil, err
				}
				out = append(out, &ASTNode{Type: "Bold", Children: nodes})
				s = rest
				continue
			}
		case strings.HasPrefix(s, "_"):
			inner, rest, ok := takeDelimited(s[1:], "_")
			if ok {
				nodes, err := parseInlines(inner)
				if err != nil {
					return nil, err
				}
				out = append(out, &ASTNode{Type: "Italic", Children: nodes})
				s = rest
				continue
			}
		case strings.HasPrefix(s, "`"):
			inner, rest, ok := takeDelimited(s[1:], "`")
			if ok {
				out = append(out, &ASTNode{Type: "Code", Value: inner})
				s = rest
				continue
			}
		case strings.HasPrefix(s, "["):
			content, after, ok := takeBracketed(s)
			if ok {
				if strings.HasPrefix(content, "^") && len(content) > 1 {
					out = append(out, &ASTNode{Type: "FootnoteRef", Attributes: Attributes{"key": content[1:]}})
					s = after
					continue
				}
				if strings.HasPrefix(content, "@") && len(content) > 1 {
					out = append(out, &ASTNode{Type: "Citation", Attributes: Attributes{"key": content[1:]}})
					s = after
					continue
				}
				node, rest, err := parseBracketSpan(content, after)
				if err != nil {
					return nil, err
				}
				out = append(out, node)
				s = rest
				continue
			}
		}
		chunk, rest := takeTextChunk(s)
		if chunk == "" && rest == s {
			// A malformed construct is literal text, not a reason to loop forever.
			chunk, rest = string(s[0]), s[1:]
		}
		out = append(out, &ASTNode{Type: "Text", Value: chunk})
		s = rest
	}
	compact := make([]*ASTNode, 0, len(out))
	for _, n := range out {
		if n.Type == "Text" && n.Value == "" {
			continue
		}
		if len(compact) > 0 && compact[len(compact)-1].Type == "Text" && n.Type == "Text" {
			compact[len(compact)-1].Value += n.Value
			continue
		}
		compact = append(compact, n)
	}
	return compact, nil
}

func parseBracketSpan(content string, after string) (*ASTNode, string, error) {
	if strings.HasPrefix(after, "(") {
		url, tail, ok := takeParen(after)
		if !ok {
			return nil, after, fmt.Errorf("invalid link")
		}
		attrs, rest, ok := extractInlineAttributes(tail)
		if !ok {
			attrs = nil
			rest = tail
		}
		linkAttrs := Attributes{"href": url}
		for k, v := range attrs {
			linkAttrs[k] = v
		}
		nodes, err := parseInlines(content)
		if err != nil {
			return nil, after, err
		}
		return &ASTNode{Type: "Link", Attributes: linkAttrs, Children: nodes}, rest, nil
	}
	attrs, rest, ok := extractInlineAttributes(after)
	if !ok {
		return &ASTNode{Type: "Text", Value: "[" + content + "]"}, after, nil
	}
	nodes, err := parseInlines(content)
	if err != nil {
		return nil, after, err
	}
	return &ASTNode{Type: "TextSpan", Attributes: attrs, Children: nodes}, rest, nil
}

func extractInlineAttributes(s string) (Attributes, string, bool) {
	if !strings.HasPrefix(s, "{") {
		return nil, s, false
	}
	block, rest, ok := takeBalanced(s, '{', '}')
	if !ok {
		return nil, s, false
	}
	attrs, err := parseAttributeBlock(block)
	if err != nil {
		return nil, s, false
	}
	return attrs, rest, true
}

func takeTextChunk(s string) (string, string) {
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			i += 2
			continue
		}
		if strings.HasPrefix(s[i:], "**") || s[i] == '_' || s[i] == '[' || s[i] == '{' {
			break
		}
		if s[i] == '`' {
			break
		}
		i++
	}
	return unescape(s[:i]), s[i:]
}

func takeDelimited(s, delim string) (string, string, bool) {
	idx := indexUnescaped(s, delim)
	if idx < 0 {
		return "", s, false
	}
	return unescape(s[:idx]), s[idx+len(delim):], true
}

func takeBracketed(s string) (string, string, bool) {
	block, rest, ok := takeBalanced(s, '[', ']')
	if !ok {
		return "", s, false
	}
	return block, rest, true
}

func takeParen(s string) (string, string, bool) {
	block, rest, ok := takeBalanced(s, '(', ')')
	if !ok {
		return "", s, false
	}
	return block, rest, true
}

func takeBalanced(s string, open, close byte) (string, string, bool) {
	if len(s) == 0 || s[0] != open {
		return "", s, false
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			continue
		}
		switch s[i] {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return s[1:i], s[i+1:], true
			}
		}
	}
	return "", s, false
}

func splitLines(src string) []string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	return strings.Split(src, "\n")
}

func countIndent(s string) int {
	n := 0
	for n < len(s) && s[n] == ' ' {
		n++
	}
	return n
}

func isHeadingLine(s string) bool {
	return len(s) > 0 && s[0] == '#' && (len(s) == 1 || s[1] == ' ' || s[1] == '#')
}

func countPrefix(s string, ch byte) int {
	i := 0
	for i < len(s) && s[i] == ch {
		i++
	}
	return i
}

func isBlockAttrLine(s string) bool {
	return strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")
}

func parseAttributeBlock(s string) (Attributes, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	return parseAttributeFields(splitFields(s))
}

func parseAttributeFields(fields []string) (Attributes, error) {
	if len(fields) == 0 {
		return nil, nil
	}
	attrs := Attributes{}
	for _, f := range fields {
		if f == "" {
			continue
		}
		switch {
		case strings.HasPrefix(f, "#"):
			attrs["id"] = f[1:]
		case strings.HasPrefix(f, "."):
			cls := f[1:]
			if prev, ok := attrs["class"]; ok && prev != "" {
				attrs["class"] = fmt.Sprint(prev) + " " + cls
			} else {
				attrs["class"] = cls
			}
		default:
			k, v, ok := strings.Cut(f, "=")
			if !ok {
				attrs[k] = true
				continue
			}
			attrs[k] = parseScalar(v)
		}
	}
	return attrs, nil
}

func parseScalar(s string) any {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"`)
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && strings.ContainsAny(s, ".eE") {
		return f
	}
	return s
}

func splitFields(s string) []string {
	var out []string
	var cur strings.Builder
	inQuote := byte(0)
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '\\' && i+1 < len(s) {
			cur.WriteByte(s[i+1])
			i++
			continue
		}
		if inQuote != 0 {
			if ch == inQuote {
				inQuote = 0
			} else {
				cur.WriteByte(ch)
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			inQuote = ch
			continue
		}
		if unicode.IsSpace(rune(ch)) {
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
			continue
		}
		cur.WriteByte(ch)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func extractTrailingAttributes(s string) (string, Attributes) {
	s = strings.TrimSpace(s)
	if !strings.HasSuffix(s, "}") {
		return s, nil
	}
	start := strings.LastIndex(s, "{")
	if start < 0 {
		return s, nil
	}
	// `]{...}` belongs to an inline span or link, even when it happens to be
	// the final construct in a heading or paragraph.
	if start > 0 && s[start-1] == ']' {
		return s, nil
	}
	attrPart := s[start:]
	attrs, err := parseAttributeBlock(attrPart)
	if err != nil {
		return s, nil
	}
	content := strings.TrimSpace(s[:start])
	if content == "" {
		return content, attrs
	}
	return content, attrs
}

func mergeAttributes(node *ASTNode, attrs Attributes) {
	if node == nil || len(attrs) == 0 {
		return
	}
	if node.Attributes == nil {
		node.Attributes = Attributes{}
	}
	for k, v := range attrs {
		node.Attributes[k] = v
	}
}

func unescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			b.WriteByte(s[i+1])
			i++
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func indexUnescaped(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		if strings.HasPrefix(s[i:], substr) {
			return i
		}
	}
	return -1
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		j := i
		for j > 0 && keys[j-1] > keys[j] {
			keys[j-1], keys[j] = keys[j], keys[j-1]
			j--
		}
	}
	return keys
}

func MarshalAST(n *ASTNode) ([]byte, error) { return json.MarshalIndent(n, "", "  ") }

func Compile(src string) (string, *ASTNode, error) {
	ast, err := Parse(src)
	if err != nil {
		return "", nil, err
	}
	html, err := RenderHTML(ast)
	if err != nil {
		return "", nil, err
	}
	return html, ast, nil
}

// CompileFile compiles a plain SMD file.
func CompileFile(path, title string) (string, *ASTNode, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	return CompileDocument(string(data), title)
}

// CompileDocument wraps compiled SMD in a standalone, print-friendly HTML page.
func CompileDocument(src, title string) (string, *ASTNode, error) {
	body, ast, err := Compile(src)
	if err != nil {
		return "", nil, err
	}
	if title == "" {
		title = "SMD document"
	}
	page := "<!doctype html><html lang=\"ja\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>" + html.EscapeString(title) + "</title><style>" + scholarlyCSS + "</style></head><body><main class=\"smd-paper\">" + body + "</main></body></html>"
	return page, ast, nil
}

const scholarlyCSS = `
@page { size: A4; margin: 24mm 20mm; }
body { color: #181818; font-family: "Noto Serif JP", "Yu Mincho", serif; line-height: 1.9; }
.smd-paper { max-width: 42em; margin: 3rem auto; }
h1, h2, h3 { line-height: 1.45; margin-top: 2.2em; }
.heading-number { font-variant-numeric: tabular-nums; }
.table-of-contents { border: 1px solid #bbb; padding: 1rem 1.5rem; margin: 2rem 0; }
.table-of-contents ol { padding-left: 1.4rem; }.toc-level-2 { margin-left: 1rem; }.toc-level-3 { margin-left: 2rem; }
figure { margin: 2rem auto; } figure img { display: block; max-width: 100%; height: auto; margin: auto; } figcaption { margin-top: .6rem; text-align: center; }
.footnotes, .bibliography { border-top: 1px solid #999; margin-top: 3rem; padding-top: 1rem; font-size: .92em; }
.citation, .footnote-ref a { text-decoration: none; } @media print { .smd-paper { max-width: none; margin: 0; } a { color: inherit; } }
`

func extractDefinitions(src string) (string, []*ASTNode, error) {
	var body, definitions []string
	var nodes []*ASTNode
	for _, line := range splitLines(src) {
		trimmed := strings.TrimSpace(line)
		kind := ""
		keyStart := 2
		if strings.HasPrefix(trimmed, "[^") {
			kind = "FootnoteDefinition"
		} else if strings.HasPrefix(trimmed, "[@") {
			kind = "BibliographyEntry"
		}
		if kind == "" {
			body = append(body, line)
			continue
		}
		end := strings.Index(trimmed, "]:")
		if end < keyStart {
			body = append(body, line)
			continue
		}
		key := trimmed[keyStart:end]
		content := strings.TrimSpace(trimmed[end+2:])
		children, err := parseInlines(content)
		if err != nil {
			return "", nil, err
		}
		nodes = append(nodes, &ASTNode{Type: kind, Attributes: Attributes{"key": key}, Children: children})
		definitions = append(definitions, key)
	}
	_ = definitions
	return strings.Join(body, "\n"), nodes, nil
}


func analyzeDocument(document *ASTNode) {
	footnotes := map[string]*ASTNode{}
	bibliography := map[string]*ASTNode{}
	figures := map[string]*ASTNode{}
	var headings []*ASTNode
	var tocNodes []*ASTNode
	footnoteNumber, bibliographyNumber, figureNumber := 0, 0, 0

	walkNodes(document, func(node *ASTNode) {
		switch node.Type {
		case "FootnoteDefinition":
			footnoteNumber++
			node.Attributes["number"] = footnoteNumber
			footnotes[fmt.Sprint(node.Attributes["key"])] = node
		case "BibliographyEntry":
			bibliographyNumber++
			node.Attributes["number"] = bibliographyNumber
			bibliography[fmt.Sprint(node.Attributes["key"])] = node
		case "Heading":
			headings = append(headings, node)
		case "Directive":
			if node.Name == "figure" {
				node.Attributes = ensureAttrs(node.Attributes)
				figureNumber++
				node.Attributes["number"] = figureNumber
				if _, exists := node.Attributes["id"]; !exists {
					node.Attributes["id"] = fmt.Sprintf("fig-%d", figureNumber)
				}
				figures[fmt.Sprint(node.Attributes["id"])] = node
			}
			if node.Name == "toc" {
				tocNodes = append(tocNodes, node)
			}
		}
	})

	sections := make([]int, 6)
	for _, heading := range headings {
		level := heading.Level
		if level < 1 || level > len(sections) {
			continue
		}
		sections[level-1]++
		for i := level; i < len(sections); i++ {
			sections[i] = 0
		}
		parts := make([]string, 0, level)
		for _, value := range sections[:level] {
			if value > 0 {
				parts = append(parts, strconv.Itoa(value))
			}
		}
		number := strings.Join(parts, ".")
		heading.Attributes = ensureAttrs(heading.Attributes)
		heading.Attributes["number"] = number
		if _, exists := heading.Attributes["id"]; !exists {
			heading.Attributes["id"] = "section-" + strings.ReplaceAll(number, ".", "-")
		}
	}
	headingsByID := map[string]*ASTNode{}
	for _, heading := range headings {
		headingsByID[fmt.Sprint(heading.Attributes["id"])] = heading
	}

	walkNodes(document, func(node *ASTNode) {
		key := ""
		switch node.Type {
		case "FootnoteRef":
			key = fmt.Sprint(node.Attributes["key"])
			if definition, ok := footnotes[key]; ok {
				node.Attributes["number"] = definition.Attributes["number"]
			} else {
				node.Attributes["number"] = "?"
			}
		case "Citation":
			key = fmt.Sprint(node.Attributes["key"])
			if figure, ok := figures[key]; ok {
				node.Type = "CrossReference"
				node.Attributes["target"] = key
				node.Attributes["label"] = "図"
				node.Attributes["number"] = figure.Attributes["number"]
			} else if heading, ok := headingsByID[key]; ok {
				node.Type = "CrossReference"
				node.Attributes["target"] = key
				node.Attributes["label"] = "節"
				node.Attributes["number"] = heading.Attributes["number"]
			} else if definition, ok := bibliography[key]; ok {
				node.Attributes["number"] = definition.Attributes["number"]
			} else {
				node.Attributes["number"] = "?"
			}
		}
	})
	for _, toc := range tocNodes {
		toc.Type = "TableOfContents"
		toc.Children = append([]*ASTNode(nil), headings...)
	}
}

func ensureAttrs(attrs Attributes) Attributes {
	if attrs == nil {
		return Attributes{}
	}
	return attrs
}

func walkNodes(node *ASTNode, visit func(*ASTNode)) {
	visit(node)
	for _, child := range node.Children {
		walkNodes(child, visit)
	}
}

func plainText(node *ASTNode) string {
	if node.Type == "Text" {
		return node.Value
	}
	var b strings.Builder
	for _, child := range node.Children {
		b.WriteString(plainText(child))
	}
	return b.String()
}

func renderFigure(b *strings.Builder, node *ASTNode) {
	attrs := filteredAttrs(node.Attributes, "src")
	attrs = filteredAttrs(attrs, "alt")
	attrs = filteredAttrs(attrs, "number")
	b.WriteString("<figure")
	writeHTMLAttrs(b, attrs)
	b.WriteString(">")
	if src, ok := node.Attributes["src"]; ok {
		b.WriteString(`<img src="`)
		b.WriteString(html.EscapeString(fmt.Sprint(src)))
		b.WriteString(`" alt="`)
		b.WriteString(html.EscapeString(fmt.Sprint(node.Attributes["alt"])))
		b.WriteString(`">`)
	}
	b.WriteString("<figcaption>図 ")
	b.WriteString(html.EscapeString(fmt.Sprint(node.Attributes["number"])))
	b.WriteString(". ")
	for _, child := range node.Children {
		_ = renderHTML(b, child)
	}
	b.WriteString("</figcaption></figure>")
}

func renderEndnotes(b *strings.Builder, document *ASTNode) {
	var notes []*ASTNode
	walkNodes(document, func(node *ASTNode) {
		if node.Type == "FootnoteDefinition" {
			notes = append(notes, node)
		}
	})
	if len(notes) == 0 {
		return
	}
	b.WriteString(`<section class="footnotes"><h2>注</h2><ol>`)
	for _, note := range notes {
		key := html.EscapeString(fmt.Sprint(note.Attributes["key"]))
		b.WriteString(`<li id="fn-` + key + `">`)
		for _, child := range note.Children {
			_ = renderHTML(b, child)
		}
		b.WriteString(` <a href="#fnref-` + key + `">↩</a></li>`)
	}
	b.WriteString(`</ol></section>`)
}

func renderBibliography(b *strings.Builder, document *ASTNode) {
	var entries []*ASTNode
	walkNodes(document, func(node *ASTNode) {
		if node.Type == "BibliographyEntry" {
			entries = append(entries, node)
		}
	})
	if len(entries) == 0 {
		return
	}
	b.WriteString(`<section class="bibliography"><h2>参考文献</h2><ol>`)
	for _, entry := range entries {
		key := html.EscapeString(fmt.Sprint(entry.Attributes["key"]))
		b.WriteString(`<li id="ref-` + key + `">`)
		for _, child := range entry.Children {
			_ = renderHTML(b, child)
		}
		b.WriteString(`</li>`)
	}
	b.WriteString(`</ol></section>`)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run smd.go <file_name.smd> [output_file.html]")
		os.Exit(1)
	}

	inputFile := os.Args[1]
	title := filepath.Base(inputFile)

	page, _, err := CompileFile(inputFile, title)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var outputFile string
	if len(os.Args) >= 3 {
		outputFile = os.Args[2]
	} else {
		ext := filepath.Ext(inputFile)
		outputFile = filepath.Join(filepath.Dir(inputFile), strings.TrimSuffix(filepath.Base(inputFile), ext)+".html")
	}

	if err := os.WriteFile(outputFile, []byte(page), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated: %s\n", outputFile)
}
