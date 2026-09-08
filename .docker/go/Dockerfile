# syntax=docker/dockerfile:1

FROM golang:1.26

WORKDIR /app

# マイグレーション管理用のCLI。イメージを再ビルドしても同じものが入るようバージョンを固定する
RUN go install github.com/pressly/goose/v3/cmd/goose@v3.27.3

# コンテナ実行時のコマンド
# execによりシェルプロセスを置き換えることで、PID 1のプロセスがアプリケーション本体となり、シグナルの伝播が正しく行われる
CMD ["sh", "-c", "go build -o /tmp/app . && exec /tmp/app"]
