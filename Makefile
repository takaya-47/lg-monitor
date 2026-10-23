.PHONY: test test_race coverage coverage_measure coverage_html migrate migrate_down migrate_status migrate_test_db migrate_test_db_down migrate_test_db_status

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

migrate:
	goose up

migrate_down:
	goose down

migrate_status:
	goose status

migrate_test_db:
	GOOSE_DBSTRING="${GOOSE_TEST_DBSTRING}" goose up

migrate_test_db_down:
	GOOSE_DBSTRING="${GOOSE_TEST_DBSTRING}" goose down

migrate_test_db_status:
	GOOSE_DBSTRING="${GOOSE_TEST_DBSTRING}" goose status