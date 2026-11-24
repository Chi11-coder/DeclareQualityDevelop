# dqd report

[verify](verify.md) の保存結果をHTML・テキスト・JSONで表示します。

## 実行例

```sh
./bin/dqd -C todo-app report -details TICKET-001
```

Makeで実行する場合は次の通りです。

```sh
make report PROJECT=todo-app TICKET_ID=TICKET-001 ARGS=-details
```

| 引数・フラグ | 意味 |
| --- | --- |
| `TICKET_ID` | 必須。表示するチケットのID |
| `-format=html` | 既定。HTMLを生成し絶対パスを表示 |
| `-format=text` | 合否の一覧を標準出力へ表示 |
| `-format=json` | 結果のJSONを標準出力へ表示 |
| `-details` | HTMLにコマンド・作業ディレクトリ・ログを含める |

`ARGS=-details` はMakeに渡すフラグです。設定ファイルの編集は不要です。`todo-app/reports/TICKET-001.html` をブラウザーで開きます。要約だけ見たいときは詳細フラグを省略してください。結果の更新は `verify` を再実行してから行います。

[コマンド一覧](README.md)
