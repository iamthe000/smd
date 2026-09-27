# SMD (Structured Markdown Dialect)

SMDは、Markdownを基にした文書形式です。見出し、脚注、文献引用、図版、LaTeX数式を記述し、印刷用のHTMLに変換できます。

## 必要な環境

* Go 1.24以降
* PDFを作る場合は、生成したHTMLを開くブラウザ

## 使い方

```bash
go run smd.go input.smd output.html
```

出力先を省略すると、入力ファイルと同じ場所に拡張子を`.html`に変えたファイルを作成します。生成したHTMLはブラウザで開き、印刷画面からPDFに保存できます。

用紙サイズは`--page-size`で指定できます。標準サイズのほか、`幅x高さ`の形式で指定できます。

```bash
go run smd.go --page-size B5 input.smd output.html
go run smd.go --page-size 210mmx297mm input.smd output.html
```

利用できる標準サイズは`A3`、`A4`、`A5`、`B4`、`B5`、`Letter`、`Legal`です。指定したサイズは生成HTMLの`@page`に反映されます。

ChromiumまたはGoogle Chromeがインストールされていれば、HTMLを経由してPDFまで一度に作成できます。

```bash
go run smd.go --pdf --page-size B5 input.smd output.pdf
```

出力先を省略した場合は、入力ファイルと同じ場所に同名のPDFを作成します。PDF出力ではヘッドレスブラウザがMathJax、CSS、画像を処理します。

記法の確認には、リポジトリにあるテスト用原稿を使えます。

```bash
go run smd.go FORTEST.smd test.html
```

テストと静的チェックは次のコマンドで実行します。

```bash
go test ./...
go vet ./...
```

## 記法

`#`から始まる行は見出しになります。`::: toc`で目次を挿入できます。見出しには節番号とアンカーが付きます。

脚注は本文に `\[^note]`、定義に `\[^note]: 注釈` と書きます。文献は `\[@key]` と定義行 `\[@key]: 書誌情報` を使います。

図版は次のように記述します。

```smd
::: figure id=fig-example src="example.jpg" alt="図の説明"
図のキャプション
:::
```

本文から `\[@fig-example]` で図を参照できます。図版は自動で番号付けされます。

インライン数式は `$E=mc^2$` または `\\(E=mc^2\\)`、独立した数式は `$$...$$` または `\\\[...\\]` で囲みます。数式はLaTeXとしてMathJax 3で表示します。

テーブルは次のように書けます。区切り行の`:`で列の揃え方を指定できます。

```smd
| 名前 | 点数 |
| :--- | ---: |
| Alice | 100 |
```

## 実装

`Parse`が原稿をASTに変換し、`analyzeDocument`が見出し・脚注・文献・図版の番号と参照先を設定します。`RenderHTML`がASTをHTMLに変換します。

主なAPIは次のとおりです。

* `Compile`：HTML断片を返す
* `CompileDocument`：CSSとMathJaxを含むHTML文書を返す
* `CompileDocumentWithPageSize`：用紙サイズを指定してHTML文書を返す
* `CompileFile`：ファイルを読み込んで変換する
* `Parse`：ASTを返す

PDF出力はCLIの`--pdf`で行います。ブラウザの実行ファイルを指定する場合は、環境変数`SMD\_BROWSER`を使います。

数式表示にはCDN上のMathJaxを使うため、生成HTMLを開くときにインターネット接続が必要です。
オフラインで生成する場合は、MathJaxの`tex-mml-chtml.js`を指定できます。

```bash
go run smd.go --mathjax-local /path/to/mathjax/es5/tex-mml-chtml.js input.smd output.html
```

APIでは`DocumentOptions{MathJaxLocalPath: "..."}`を使えます。`DisableMathJaxCDN: true`ならローカルのみ、`false`ならCDNの読み込みに失敗したときローカルへフォールバックします。

## ファイル

* `smd.go`：パーサ、AST、HTMLレンダラ、CLI
* `smd\_test.go`：Goのテスト
* `FORTEST.smd`：記法を確認する原稿
* `examples/city.jpg`：図版サンプル。CC0の画像

