//go:build ignore

// このプログラムは ent スキーマから goose 形式のバージョン管理マイグレーションを生成する。
//
// 使い方:
//
//	ATLAS_DEV_URL="mysql://root:pass@localhost:3306/dev_schema" \
//	  go run -mod=mod ./ent/migrate <migration_name>
//
// dev データベース（ATLAS_DEV_URL）は Atlas がスキーマ差分計算に使う一時DBで、
// 実データを持たない専用スキーマを指定すること。
package main

import (
	"context"
	"log"
	"os"

	"ariga.io/atlas/sql/sqltool"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql/schema"
	_ "github.com/go-sql-driver/mysql"

	"github.com/ITK13201/CalendarRabbit/backend/ent/migrate"
)

func main() {
	ctx := context.Background()

	if len(os.Args) != 2 {
		log.Fatalln("migration name is required. Usage: go run -mod=mod ./ent/migrate <name>")
	}

	devURL := os.Getenv("ATLAS_DEV_URL")
	if devURL == "" {
		log.Fatalln("ATLAS_DEV_URL is required (e.g. mysql://root:pass@localhost:3306/dev_schema)")
	}

	dir, err := sqltool.NewGooseDir("db/migrations")
	if err != nil {
		log.Fatalf("failed creating goose migration directory: %v", err)
	}

	opts := []schema.MigrateOption{
		schema.WithDir(dir),
		schema.WithMigrationMode(schema.ModeReplay),
		schema.WithDialect(dialect.MySQL),
		schema.WithFormatter(sqltool.GooseFormatter),
	}

	if err := migrate.NamedDiff(ctx, devURL, os.Args[1], opts...); err != nil {
		log.Fatalf("failed generating migration file: %v", err)
	}

	log.Printf("migration %q generated in db/migrations", os.Args[1])
}
