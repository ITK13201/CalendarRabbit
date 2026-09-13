// Package testsupport はテスト用の DB クライアント生成ヘルパを提供する。
package testsupport

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/ent"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/persistence"
	_ "github.com/go-sql-driver/mysql"
)

// defaultAdminDSN は DB を作成できる管理者接続の既定 DSN（データベース名なし）。
// docker-compose / CI いずれの MySQL でも利用できる root 接続。
const defaultAdminDSN = "root:rootpassword@tcp(127.0.0.1:3306)/?parseTime=true&loc=UTC&charset=utf8mb4"

// adminDSN は環境変数 TEST_MYSQL_DSN を優先して管理者 DSN を返す。
func adminDSN() string {
	if v := os.Getenv("TEST_MYSQL_DSN"); v != "" {
		return v
	}
	return defaultAdminDSN
}

// testDBName はテストプロセスごとに一意な DB 名を返す（パッケージ間の並行実行で衝突しない）。
func testDBName() string {
	return fmt.Sprintf("cr_test_%d", os.Getpid())
}

// NewClient はテスト専用の隔離されたデータベースを用意し、ent クライアントを返す。
// DB へ接続できない場合はテストをスキップする（DB 非依存の環境向け）。
func NewClient(t *testing.T) *ent.Client {
	t.Helper()

	admin, err := sql.Open("mysql", adminDSN())
	if err != nil {
		t.Skipf("skip: cannot open admin DB: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := admin.PingContext(ctx); err != nil {
		_ = admin.Close()
		t.Skipf("skip: test MySQL is not available: %v", err)
	}

	dbName := testDBName()
	// プロセス専用DBを作り直す（前回の残骸を除去して隔離を保証）。
	if _, err := admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+dbName); err != nil {
		_ = admin.Close()
		t.Fatalf("drop test db: %v", err)
	}
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+dbName+" CHARACTER SET utf8mb4"); err != nil {
		_ = admin.Close()
		t.Fatalf("create test db: %v", err)
	}

	client, err := persistence.Open(dsnForDB(dbName))
	if err != nil {
		_ = admin.Close()
		t.Fatalf("open ent client: %v", err)
	}
	if err := client.Schema.Create(ctx); err != nil {
		_ = client.Close()
		_ = admin.Close()
		t.Fatalf("migrate test db: %v", err)
	}

	t.Cleanup(func() {
		_ = client.Close()
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer dropCancel()
		_, _ = admin.ExecContext(dropCtx, "DROP DATABASE IF EXISTS "+dbName)
		_ = admin.Close()
	})

	return client
}

// dsnForDB は指定DB名の ent 用 DSN を返す。TEST_MYSQL_DSN 指定時はその接続情報を流用する。
func dsnForDB(dbName string) string {
	// admin DSN の "/" 直後（データベース位置）に dbName を差し込む。
	base := adminDSN()
	// base 形式: user:pass@tcp(host:port)/?params
	for i := 0; i < len(base); i++ {
		if base[i] == '/' {
			// "tcp(...)" 内の '/' は無いため最初の '/' がDB位置。
			return base[:i+1] + dbName + base[i+1:]
		}
	}
	return base + "/" + dbName
}
