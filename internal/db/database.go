package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Connectはデータベースへの疎通を確認し、接続可能な場合はコネクションプールを返します。
func Connect(ctx context.Context) (*sql.DB, error) {
	// DSNの検証
	db, err := sql.Open("mysql", os.Getenv("DB_DSN"))
	if err != nil {
		return nil, fmt.Errorf("dsn is invalid: %w", err)
	}

	db.SetConnMaxLifetime(3 * time.Minute) // MySQLサーバへの接続の寿命。経過後は接続を最初からやり直す。
	db.SetMaxOpenConns(10)                 // コネクションプールに対して同時に開くことができる最大接続数
	db.SetMaxIdleConns(10)                 // コネクションプールがアイドル状態で保持する接続の最大数

	// 接続チェック
	err = db.PingContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("error when connecting to database: %w", err)
	}

	return db, nil
}
