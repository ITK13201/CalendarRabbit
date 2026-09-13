.PHONY: build test run generate migrate-gen migrate-up migrate-down swagger tidy

# ローカル開発用の既定値（docker-compose の MySQL）
DSN ?= calendarrabbit:calendarrabbit@tcp(localhost:3306)/calendarrabbit?parseTime=true&loc=UTC
ATLAS_DEV_URL ?= mysql://root:rootpassword@localhost:3306/calendarrabbit_dev

build:
	go build ./...

test:
	go test ./...

run:
	go run ./cmd/server

# ent のコード生成（versioned-migration 機能付き）
generate:
	go generate ./ent/...

# ent スキーマから goose 形式のマイグレーションを生成（要: dev DB）
# 使い方: make migrate-gen NAME=add_something
migrate-gen:
	ATLAS_DEV_URL="$(ATLAS_DEV_URL)" go run -mod=mod ./ent/migrate/main.go $(NAME)

# goose マイグレーション適用/巻き戻し（要: goose CLI）
migrate-up:
	goose -dir db/migrations mysql "$(DSN)" up

migrate-down:
	goose -dir db/migrations mysql "$(DSN)" down

# swagger ドキュメント生成
swagger:
	swag init -g cmd/server/main.go -o docs/swagger --parseInternal

tidy:
	go mod tidy
