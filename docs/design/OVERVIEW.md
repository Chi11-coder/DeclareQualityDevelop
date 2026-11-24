# DQD CLIの内部設計

操作は [導入ガイド](../GETTING_STARTED.md)、書式は [リファレンス](../GETTING_STARTED.md#定義項目を調べる) を参照してください。


## 宣言の読み込み

| コマンド | 読み込むチケット | 検証設定 |
| --- | --- | --- |
| `validate` | `tickets/` 直下の全Markdown | あれば確認 |
| `resolve`、`context`、`verify` | 指定の1枚 | `verify` で確認 |
| `quality list`、`quality show` | なし | — |
| `ticket new` | 作成する雛形 | — |
| `report` | 保存済みJSONを使用 | — |

`report` を除く表のコマンドは、品質要求と全参照先を確認します。`validate` は形式と参照の検証、`verify` は登録テストの実行です。

`init` は既存の親ディレクトリにプロジェクトを作ります。初期ファイルと名前の規則は [init](../reference/commands/init.md) を参照してください。

## 要求の選択とContext

- `always`、属性が一致する `automatic`、`quality.include` から選び、重複を除いて宣言順に並べます。
- 適用要求が0件、または指定や個別合格条件に不正なIDがある場合はエラーです。`domains` は選択に使いません。
- Contextにはチケットの機能要件、品質要求の本文と宣言情報、個別合格条件が入ります。技術構成や前の作業を引き継ぐ条件は、チケットの本文に書きます。
- Markdownの見出しをContext用に調整し、`.dqd/contexts/チケットID.md` へ一時ファイル経由で保存します。`-stdout` は表示のみ、`-force` は上書きです。

## 品質検証とレポート

- `verify` は適用した要求を宣言順に処理します。同じ要求のチェックは [verification.yml](../reference/verification.md) の記載順で実行します。
- 結果とログをプロジェクト内の `verifies/チケットID.json` に保存します。チェック未登録の要求は未確認です。
- `report` は保存済みJSONから `reports/チケットID.html` を生成します。`-details` でコマンドとログを表示し、`-format=text` / `-format=json` では標準出力へ出します。
- HTMLは `html/template` で値をエスケープします。CSPで同梱の固定CSSだけを許可し、外部リソースは読み込みません。

## 入出力と終了コード

- `-C` は対象プロジェクト、`init` の場合は既存の親ディレクトリ。省略時は現在のディレクトリです。
- DQDは `os.Root` でファイル操作の範囲を制限します。テストコマンドは指定の作業ディレクトリで通常のOS権限で動きます。
- フラグは位置引数の前に指定します。通常出力は標準出力、エラーは標準エラー出力です。

| CLIの終了コード | 意味 |
| --- | --- |
| `0` | 成功・ヘルプ |
| `1` | 宣言や入出力のエラー、または `verify` で全件合格に至らず |
| `2` | コマンドや引数の誤り |

Make経由ではMakeの終了コードが返る場合があります。[コマンド一覧](../../README.md#コマンド一覧)

## Goの配置

Go 1.27.3、モジュール名 `dqd`、ルートは `pkg/`。引数は標準の `flag.FlagSet`、YAMLは `gopkg.in/yaml.v3` で解析します。レポートのHTMLとCSSは `go:embed` で実行ファイルに組み込みます。
