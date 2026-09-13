//go:build tools

// Package tools はビルドには含まれないが、コード生成・マイグレーション生成に
// 必要な依存を go.mod に保持するためのファイル。
package tools

import (
	_ "ariga.io/atlas/sql/sqltool"
	_ "github.com/go-sql-driver/mysql"
)
