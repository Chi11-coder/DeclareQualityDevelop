# dqd init

新しいプロジェクトを作ります。

## 実行例

```sh
./bin/dqd -C . init -name todo-app
```

Makeで実行する場合は次の通りです。

```sh
make init PROJECT=. ARGS="-name todo-app"
```

| 引数・フラグ | 意味 |
| --- | --- |
| `-C PARENT` | 既存の親ディレクトリ。既定は現在のディレクトリ |
| `-name NAME` | プロジェクト名。省略時は対話入力 |

名前は英数字で始まる英数字・`-`・`_`の255文字以内です。同名のパスがあるとエラーです。

`requirements.yml`、`quality/common.md`、チェック未登録の `verification.yml`、空の `tickets/`、Git除外設定を持つ `verifies/`・`reports/` ができます。最初の品質要求は `CM-001` です。成功時は作成先の絶対パスを表示します。

[コマンド一覧](README.md)
