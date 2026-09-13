// Package persistence は ent を用いた各エンティティの永続化（CRUD）を提供する。
package persistence

import (
	"context"
	"errors"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/ITK13201/CalendarRabbit/backend/ent"
	_ "github.com/go-sql-driver/mysql"
)

// Open は DSN から ent クライアントを生成する。
func Open(dsn string) (*ent.Client, error) {
	drv, err := entsql.Open(dialect.MySQL, dsn)
	if err != nil {
		return nil, err
	}
	return ent.NewClient(ent.Driver(drv)), nil
}

// AutoMigrate は ent スキーマからテーブルを作成する（テスト・開発用途）。
// 本番は goose マイグレーションを使用する。
func AutoMigrate(ctx context.Context, client *ent.Client) error {
	return client.Schema.Create(ctx)
}

// WithTx は fn をトランザクション内で実行する。fn には tx にバインドされた
// *ent.Client が渡され、各リポジトリはこのクライアントで生成することで
// 同一トランザクションで操作できる。エラー時はロールバックする。
func WithTx(ctx context.Context, client *ent.Client, fn func(txClient *ent.Client) error) error {
	tx, err := client.Tx(ctx)
	if err != nil {
		return err
	}
	if err := fn(tx.Client()); err != nil {
		if rerr := tx.Rollback(); rerr != nil {
			return errors.Join(err, rerr)
		}
		return err
	}
	return tx.Commit()
}
