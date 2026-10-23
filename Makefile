# テスト全体
.PHONY: test
test:
	go test -v ./...

# 並行実行を含むコードをテストする際は、ここに追加しつつmake test_raceを実行する
.PHONY: test_race
test_race:
	go test -race -v ./internal/sse

.PHONY: coverage
coverage: coverage_measure coverage_html

# 通常は `make coverage` を実行すれば、カバレッジ測定とHTMLレポートの生成が行われる
.PHONY: coverage_measure
coverage_measure:
	go test ./... -cover -coverprofile=c.out

# 通常は `make coverage` を実行すれば、カバレッジ測定とHTMLレポートの生成が行われる
.PHONY: coverage_html
coverage_html:
	go tool cover -html=c.out

.PHONY: migrate
migrate:
	goose up

.PHONY: migrate_down
migrate_down:
	goose down

.PHONY: migrate_status
migrate_status:
	goose status

.PHONY: migrate_test_db
migrate_test_db:
	GOOSE_DBSTRING="${GOOSE_TEST_DBSTRING}" goose up

.PHONY: migrate_test_db_down
migrate_test_db_down:
	GOOSE_DBSTRING="${GOOSE_TEST_DBSTRING}" goose down

.PHONY: migrate_test_db_status
migrate_test_db_status:
	GOOSE_DBSTRING="${GOOSE_TEST_DBSTRING}" goose status
