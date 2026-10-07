package rdb

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Store struct {
	db *sql.DB
}

// NewStore はMySQLへの疎通を確認し、接続可能な場合はコネクションプールを返します。
func NewStore(ctx context.Context, DBDSN string) (*Store, error) {
	// DSNの検証
	db, err := sql.Open("mysql", DBDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to verify DSN: %w", err)
	}

	db.SetConnMaxLifetime(3 * time.Minute) // MySQLサーバへの接続の寿命。経過後は接続を最初からやり直す。
	db.SetMaxOpenConns(10)                 // コネクションプールに対して同時に開くことができる最大接続数
	db.SetMaxIdleConns(10)                 // コネクションプールがアイドル状態で保持する接続の最大数

	// 接続チェック
	err = db.PingContext(ctx)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	return &Store{db: db}, nil
}

// Close はデータベース接続を閉じます。
func (s *Store) Close() error {
	return s.db.Close()
}
