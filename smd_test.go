package smd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseExampleAST(t *testing.T) {
	src := `{#intro}
# Hello [World]{.text-blue}

::: alert type="warning"
Be careful!
:::`
	ast, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := json.MarshalIndent(ast, "", "  ")
	if !strings.Contains(string(got), `"type": "Document"`) {
		t.Fatalf("unexpected ast: %s", got)
	}
	if len(ast.Children) != 2 {
		t.Fatalf("expected 2 top-level nodes, got %d", len(ast.Children))
	}
	if ast.Children[0].Type != "Heading" || ast.Children[0].Attributes["id"] != "intro" {
		t.Fatalf("bad heading attrs: %#v", ast.Children[0])
	}
	if ast.Children[1].Type != "Directive" || ast.Children[1].Name != "alert" {
		t.Fatalf("bad directive: %#v", ast.Children[1])
	}
}

func TestNestedDirectivesAndComponents(t *testing.T) {
	src := `::: grid cols=3 gap=4
  ::: col span=2
    ## Main Content
    <@Card title="Product Card" price=29.99 featured>
      This is the card body text.
      <@Button label="Buy Now" variant="primary" />
    </@Card>
  :::
  ::: col span=1
    ### Sidebar
  :::
:::`
	ast, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(ast.Children) != 1 || ast.Children[0].Type != "Directive" {
		t.Fatalf("unexpected ast root: %#v", ast.Children)
	}
	grid := ast.Children[0]
	if grid.Name != "grid" || grid.Attributes["cols"] != int64(3) || grid.Attributes["gap"] != int64(4) {
		t.Fatalf("bad grid attrs: %#v", grid.Attributes)
	}
	if len(grid.Children) != 2 || grid.Children[0].Type != "Directive" {
		t.Fatalf("bad nested directives: %#v", grid.Children)
	}
}

func TestInlineSpanAttributes(t *testing.T) {
	src := `This is a [crucial warning]{.text-danger weight=bold id=warn-01}.`
	ast, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	p := ast.Children[0]
	if len(p.Children) < 2 || p.Children[1].Type != "TextSpan" {
		t.Fatalf("expected TextSpan, got %#v", p.Children)
	}
	if p.Children[1].Attributes["class"] != "text-danger" || p.Children[1].Attributes["weight"] != "bold" || p.Children[1].Attributes["id"] != "warn-01" {
		t.Fatalf("bad inline attrs: %#v", p.Children[1].Attributes)
	}
}

func TestDanglingBlockAttrs(t *testing.T) {
	src := `{#hero-section .bg-dark .p-8 fullWidth=true}
# Welcome to the Future

{.lead}
This is a paragraph.`
	ast, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if ast.Children[0].Type != "Heading" {
		t.Fatalf("expected heading")
	}
	if ast.Children[0].Attributes["id"] != "hero-section" || ast.Children[0].Attributes["class"] != "bg-dark p-8" {
		t.Fatalf("bad heading attrs: %#v", ast.Children[0].Attributes)
	}
	if ast.Children[1].Type != "Paragraph" || ast.Children[1].Attributes["class"] != "lead" {
		t.Fatalf("bad paragraph attrs: %#v", ast.Children[1])
	}
}

func TestRenderHTML(t *testing.T) {
	src := `{#intro}
# Hello [World]{.text-blue}`
	html, _, err := Compile(src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `<h1 id="intro">`) || !strings.Contains(html, `<span class="text-blue">World</span>`) {
		t.Fatalf("unexpected html: %s", html)
	}
}

func TestInlineAttributesConsumeOnlyTheirOwnSuffix(t *testing.T) {
	ast, err := Parse(`A [span]{.accent} and [link](https://example.test){target=_blank}.`)
	if err != nil {
		t.Fatal(err)
	}
	p := ast.Children[0]
	if len(p.Children) != 5 {
		t.Fatalf("expected text, span, text, link, text; got %#v", p.Children)
	}
	if p.Children[1].Type != "TextSpan" || p.Children[1].Attributes["class"] != "accent" {
		t.Fatalf("bad span: %#v", p.Children[1])
	}
	if p.Children[3].Type != "Link" || p.Children[3].Attributes["target"] != "_blank" {
		t.Fatalf("bad link: %#v", p.Children[3])
	}
}

func TestMalformedInlineSyntaxRemainsLiteral(t *testing.T) {
	ast, err := Parse(`An unfinished [span never hangs.`)
	if err != nil {
		t.Fatal(err)
	}
	if got := ast.Children[0].Children[0].Value; got != "An unfinished [span never hangs." {
		t.Fatalf("unexpected literal text: %q", got)
	}
}

func TestScholarlyDocumentFeatures(t *testing.T) {
	src := `::: toc
:::

# 序論

都市表象については [@yamada2024] を参照[^note-urban]。

::: figure id=fig-city src="city.jpg" alt="都市の景観"
都市の景観
:::

図[@fig-city]が示すように、都市空間は重要である。

[^note-urban]: 山田の議論は都市と読者の関係を扱う。
[@yamada2024]: 山田太郎『都市と文学』、2024年。
`
	page, ast, err := CompileDocument(src, "論文の例")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page, `<nav class="table-of-contents"`) ||
		!strings.Contains(page, `id="fig-city"`) ||
		!strings.Contains(page, `href="#fn-note-urban"`) ||
		!strings.Contains(page, `href="#ref-yamada2024"`) ||
		!strings.Contains(page, `href="#fig-city">図 1</a>`) ||
		!strings.Contains(page, `<section class="bibliography"`) {
		t.Fatalf("missing scholarly HTML features: %s", page)
	}
	if ast.Children[0].Type != "TableOfContents" {
		t.Fatalf("expected generated table of contents, got %s", ast.Children[0].Type)
	}
}

func TestCompileFileFromCmdComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "paper.cmd")
	src := `@echo off
setlocal

REM # 文書の題名
REM
REM 本文の段落です。

echo running
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	page, _, err := CompileFile(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page, `<h1`) || !strings.Contains(page, `本文の段落です。`) {
		t.Fatalf("cmd comments were not compiled: %s", page)
	}
	if strings.Contains(page, `echo running`) {
		t.Fatalf("batch commands should not be rendered: %s", page)
	}
}

func TestSpacePreservationAroundInlineAttributes(t *testing.T) {
	ast, err := Parse(`A [span]{.accent} and [link](https://example.test){target=_blank} is here.`)
	if err != nil {
		t.Fatal(err)
	}
	p := ast.Children[0]
	if len(p.Children) != 5 {
		t.Fatalf("expected 5 child nodes, got %d: %#v", len(p.Children), p.Children)
	}
	// Check the text nodes to ensure leading and trailing spaces are preserved
	if p.Children[0].Value != "A " {
		t.Errorf("expected 'A ', got %q", p.Children[0].Value)
	}
	if p.Children[2].Value != " and " {
		t.Errorf("expected ' and ', got %q", p.Children[2].Value)
	}
	if p.Children[4].Value != " is here." {
		t.Errorf("expected ' is here.', got %q", p.Children[4].Value)
	}

	html, err := RenderHTML(ast)
	if err != nil {
		t.Fatal(err)
	}
	expected := `<p>A <span class="accent">span</span> and <a href="https://example.test" target="_blank">link</a> is here.</p>`
	if html != expected {
		t.Errorf("expected HTML %q, got %q", expected, html)
	}
}

func TestCompileFileFromCmdDirectives(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "paper.cmd")
	src := `@echo off
setlocal

REM # Title
REM ::: toc
REM :::

:: ::: alert
:: Attention!
:: :::

::: info
REM Info message!
:::
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	page, _, err := CompileFile(path, "")
	if err != nil {
		t.Fatal(err)
	}
	// Verify that the table-of-contents nav and the nested custom directives are properly compiled in HTML
	if !strings.Contains(page, `<nav class="table-of-contents"`) {
		t.Errorf("missing TOC in output: %s", page)
	}
	if !strings.Contains(page, `data-directive="alert"`) {
		t.Errorf("missing alert directive in output: %s", page)
	}
	if !strings.Contains(page, `data-directive="info"`) {
		t.Errorf("missing info directive in output: %s", page)
	}
	if !strings.Contains(page, `Info message!`) {
		t.Errorf("missing directive content in output: %s", page)
	}
}
