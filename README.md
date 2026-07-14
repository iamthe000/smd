# SMD (Structured Markdown Dialect)

SMD は、Markdown の読みやすさと LaTeX の論文向け機能を両立するための文書形式です。
原稿から、見出し番号・目次・脚注・引用・参考文献・図版番号を含む、印刷用 CSS 付きの HTML を生成します。

## 必要な環境

- Go 1.24 以降
- PDF を作る場合は、生成した HTML を開けるブラウザ

## 動かし方

`.smd` ファイルを HTML に変換するには、リポジトリのルートで次のように実行します。

```bash
go run smd.go README.smd
```

または、出力先の HTML ファイルを明示することもできます。

```bash
go run smd.go README.smd output.html
```

生成された `README.html` をブラウザで開き、印刷から PDF に保存します。

テストを実行するには次を使います。

```bash
go test ./...
go vet ./...
```

## 原稿の書き方

```smd
::: toc
:::

# 序論

都市表象については [@yamada2024] を参照[^note-urban]。

::: figure id=fig-city src="city.jpg" alt="都市の景観"
都市の景観
:::

図[@fig-city]が示すように、都市空間は重要である。

[^note-urban]: 山田の議論は都市と読者の関係を扱う。
[@yamada2024]: 山田太郎『都市と文学』、2024年。
```

### 見出しと目次

```smd
::: toc
:::

# 第一章
## 第一節
```

- `::: toc` は番号付きの目次を出力します。
- `#`、`##`、`###` の見出しには自動で節番号とアンカーが付きます。

### 脚注

本文では `[^キー]`、原稿内の任意の場所に定義を書きます。

```smd
本文の注釈[^note-1]。

[^note-1]: 注釈の本文です。
```

### 文献引用

本文では `[@キー]`、原稿内の任意の場所に文献情報を書きます。

```smd
先行研究を参照する [@tanaka2025]。

[@tanaka2025]: 田中一郎『日本近代史研究』、2025年。
```

### 図版と相互参照

```smd
::: figure id=fig-map src="map.png" alt="研究対象地域の地図"
研究対象地域
:::

図[@fig-map]を参照してください。
```

- `id` は図を参照するための固有名です。
- `src` は画像ファイルのパスです。
- 図は自動で番号付けされます。
- `[@fig-map]` は「図 1」のようなリンクになります。

## Go から使う

```go
package main

import (
    "os"

    "smd"
)

func main() {
    source := "# タイトル\n\n本文です。"
    page, _, err := smd.CompileDocument(source, "論文タイトル")
    if err != nil {
        panic(err)
    }
    if err := os.WriteFile("paper.html", []byte(page), 0644); err != nil {
        panic(err)
    }
}
```

- `CompileDocument` は A4 印刷用 CSS 付きの完全な HTML 文書を返します。
- `CompileFile` は `.smd` ファイルを直接読み込んで変換します。
- `Compile` は HTML 断片だけが必要な場合に使います。
- `Parse` は SMD の AST が必要な場合に使います。

## PDF にする

1. `CompileDocument` の結果を `paper.html` として保存します。
2. ブラウザで `paper.html` を開きます。
3. 印刷画面から「PDF に保存」を選びます。

この方式なら LaTeX の導入なしで、日本語フォントをブラウザ側の設定で選択できます。
