# dqd validate

宣言とファイルの整合性を調べます。

## 実行例

```sh
./bin/dqd -C docs/example validate
```

Makeで実行する場合は次の通りです。

```sh
make validate PROJECT=docs/example
```

位置引数はありません。`requirements.yml` と全参照文書、`tickets/` 直下のMarkdown、存在する場合は `verification.yml` の形式と参照を確認します。サンプルで成功すると `Valid: 5 requirements, 3 tickets` と表示します。テストの実行は [verify](verify.md) を使います。

[コマンド一覧](README.md)
