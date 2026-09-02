# lg-monitor
自治体HP監視ツール

## セットアップ
### 1. 環境変数ファイルの作成
`.env` は git 管理外なので、clone 後に雛形からコピーして作成します。

```bash
cp .env.example .env
```

### 2. コンテナの起動
```bash
docker compose up -d
```

### 3. マイグレーションの実行
```bash
docker compose exec app goose up
```

### 4. マスタデータの投入
`prefectures`、`municipalities`、`monitor_targets` に初期データを投入します。
`prefectures`のみ`seed/prefectures.sql`にSQLを用意しています。