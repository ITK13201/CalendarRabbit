// Package db は goose マイグレーションファイルを埋め込みで提供する。
package db

import "embed"

// MigrationsFS は db/migrations 配下の SQL マイグレーションを埋め込む。
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS

// MigrationsDir は埋め込み FS 内のマイグレーションディレクトリ。
const MigrationsDir = "migrations"
