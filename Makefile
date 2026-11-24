.DEFAULT_GOAL := help

GO ?= go
MODULE_DIR := pkg
BINARY ?= bin/dqd
PROJECT ?= .
TICKET_ID ?= TICKET-001
QUALITY_ID ?= UI-001
ARGS ?=

.PHONY: help build dqd init validate resolve context verify report quality-list quality-show ticket-new test vet fmt check

# 利用できるターゲットと、対象プロジェクトなどの指定方法を表示する。
help:
	@printf '%s\n' \
	  'DQD CLI（Go 1.27.3）' \
	  'make build                              CLIをbin/dqdへビルド' \
	  'make dqd ARGS="-h"                     dqdへ任意の引数を渡す' \
	  'make init                              名前を尋ねてプロジェクトを作成' \
	  '  ARGS="-name todo-app" で名前を指定、PROJECTは作成先の親ディレクトリ' \
	  'make validate PROJECT=/path/to/project  宣言の整合性を検証' \
	  'make resolve PROJECT=... TICKET_ID=...  適用する品質要求を表示' \
	  'make context PROJECT=... TICKET_ID=...  Contextを生成' \
	  '  ARGS=-stdout で表示のみ、ARGS=-force で既存Contextを更新' \
	  'make verify PROJECT=... TICKET_ID=...   チェックを実行して結果を保存' \
	  'make report PROJECT=... TICKET_ID=...   保存した結果をHTMLに出力' \
	  '  ARGS=-details でHTMLにコマンドとログも表示' \
	  '  ARGS=-format=text または ARGS=-format=json で端末に表示' \
	  'make quality-list PROJECT=...          品質要求を一覧表示' \
	  'make quality-show PROJECT=... QUALITY_ID=...  品質要求の本文を表示' \
	  'make ticket-new PROJECT=... TICKET_ID=... ARGS=...  チケット雛形を作成' \
	  'make test / vet / fmt / check           テスト・静的解析・整形'

# Goのビルドキャッシュを利用し、内部パッケージを含む変更を反映してビルドする。
build:
	@mkdir -p "$(dir $(BINARY))"
	$(GO) -C "$(MODULE_DIR)" build -o "$(abspath $(BINARY))" ./cmd/dqd

# ビルドしたCLIに、共通のプロジェクト指定と任意の引数を渡す。
dqd: build
	"$(abspath $(BINARY))" -C "$(PROJECT)" $(ARGS)

# 名前を尋ね、PROJECTを親とする新しいプロジェクトを作成する。
init: build
	"$(abspath $(BINARY))" -C "$(PROJECT)" init $(ARGS)

# 品質要求と全チケットの整合性を検証する。
validate: build
	"$(abspath $(BINARY))" -C "$(PROJECT)" validate $(ARGS)

# 指定したチケットに適用する品質要求を解決する。
resolve: build
	"$(abspath $(BINARY))" -C "$(PROJECT)" resolve $(ARGS) "$(TICKET_ID)"

# 指定したチケットのContextを生成する。追加フラグはARGSで渡す。
context: build
	"$(abspath $(BINARY))" -C "$(PROJECT)" context $(ARGS) "$(TICKET_ID)"

# チケットに対応するチェックを実行し、検証結果を保存する。
verify: build
	"$(abspath $(BINARY))" -C "$(PROJECT)" verify $(ARGS) "$(TICKET_ID)"

# 保存済みの検証結果からHTMLレポートを作成する。
report: build
	"$(abspath $(BINARY))" -C "$(PROJECT)" report $(ARGS) "$(TICKET_ID)"

# 品質要求の一覧を表示する。
quality-list: build
	"$(abspath $(BINARY))" -C "$(PROJECT)" quality list $(ARGS)

# 指定した品質IDに対応するMarkdown本文を表示する。
quality-show: build
	"$(abspath $(BINARY))" -C "$(PROJECT)" quality show $(ARGS) "$(QUALITY_ID)"

# タイトル・領域・品質IDなどをARGSから渡して、チケット雛形を作成する。
ticket-new: build
	"$(abspath $(BINARY))" -C "$(PROJECT)" ticket new $(ARGS) "$(TICKET_ID)"

# 既存の単体テストとCLIの統合テストを実行する。
test:
	$(GO) -C "$(MODULE_DIR)" test ./...

# Goの静的解析を実行する。
vet:
	$(GO) -C "$(MODULE_DIR)" vet ./...

# pkg内のcmdとinternalを含む全パッケージのGoファイルを標準形式に整形する。
fmt:
	$(GO) -C "$(MODULE_DIR)" fmt ./...

# テストと静的解析をまとめて実行する。
check: test vet
