package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"smd"
)

func main() {
	input := flag.String("input", "", "input SMD, .cmd, or .bat file")
	output := flag.String("output", "", "output HTML file")
	title := flag.String("title", "", "document title")
	dumpAST := flag.Bool("ast", false, "print the AST after compilation")
	flag.Parse()

	if *input != "" {
		if *title == "" {
			*title = filepath.Base(*input)
		}
		page, ast, err := smd.CompileFile(*input, *title)
		if err != nil {
			log.Fatal(err)
		}
		if *dumpAST {
			json, err := smd.MarshalAST(ast)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(string(json))
		}
		if *output == "" {
			ext := filepath.Ext(*input)
			*output = filepath.Join(filepath.Dir(*input), strings.TrimSuffix(filepath.Base(*input), ext)+".html")
		}
		if err := os.WriteFile(*output, []byte(page), 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Println(*output)
		return
	}

	source := `::: toc
:::

# 序論

都市表象については [@yamada2024] を参照[^note-urban]。

::: figure id=fig-city src="city.jpg" alt="都市の景観"
都市の景観
:::`

	// Definitions may appear anywhere in the manuscript.
	source += `

図[@fig-city]が示すように、都市空間は重要である。

[^note-urban]: 山田の議論は都市と読者の関係を扱う。
[@yamada2024]: 山田太郎『都市と文学』、2024年。`

	html, ast, err := smd.CompileDocument(source, "論文の例")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Standalone HTML:")
	fmt.Println(html)
	fmt.Println("\nAST:")
	json, err := smd.MarshalAST(ast)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(json))
}
