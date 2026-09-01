# syntax=docker/dockerfile:1

FROM golang:1.26

WORKDIR /app

# マイグレーション管理用のCLI。イメージを再ビルドしても同じものが入るようバージョンを固定する
RUN go install github.com/pressly/goose/v3/cmd/goose@v3.27.3

CMD ["go", "run", "."]
