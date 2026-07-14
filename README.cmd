@echo off
setlocal

REM # SMD を `.cmd` から HTML / PDF にする
REM
REM このファイルは 2 つの役割を持ちます。
REM
REM 1. Windows でそのまま実行できる。
REM 2. コメント行だけを取り出せば、そのまま SMD の原稿になる。
REM
REM ## 変換方法
REM
REM `go run ./cmd/smd-demo -input README.cmd`
REM
REM 生成された `README.html` をブラウザで開き、印刷から PDF に保存します。
REM
REM ## 書き方
REM
REM `REM` の後ろに SMD を書きます。コマンド行は HTML 化の対象になりません。
REM
REM # 序論
REM
REM 都市表象については [@yamada2024] を参照[^note-urban]。
REM
REM [^note-urban]: 山田の議論は都市と読者の関係を扱う。
REM [@yamada2024]: 山田太郎『都市と文学』、2024年。
REM
REM ## 実行部
REM
REM この下は実行用です。

cd /d "%~dp0"

where go >nul 2>nul
if errorlevel 1 (
  echo Go is not installed or is not on PATH.
  echo Install Go 1.24 or later, then run this file again.
  exit /b 1
)

go run .\cmd\smd-demo -input README.cmd -title "SMD README"
exit /b %errorlevel%
