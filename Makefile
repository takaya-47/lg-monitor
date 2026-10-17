.PHONY: test test_race coverage coverage_measure coverage_html

# テスト全体
test:
	go test -v ./...

# 並行実行を含むコードをテストする際は、ここに追加しつつmake test_raceを実行する
test_race:
	go test -race -v ./internal/sse

coverage: coverage_measure coverage_html

# 通常は `make coverage` を実行すれば、カバレッジ測定とHTMLレポートの生成が行われる
coverage_measure:
	go test ./... -cover -coverprofile=c.out

# 通常は `make coverage` を実行すれば、カバレッジ測定とHTMLレポートの生成が行われる
coverage_html:
	go tool cover -html=c.out