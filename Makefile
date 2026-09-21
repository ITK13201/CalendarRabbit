# CalendarRabbit ルート Makefile（op + docker compose デバッグ用）
#
# 秘密が必要なコマンド（up/restart 等）は 1Password から注入して実行する。
# 秘密が不要なコマンド（down/ps/logs 等）はダミーキーで即実行し、op 認証を求めない。

ENV_FILE := .env.op
COMPOSE  := docker compose
# 秘密を 1Password から注入して compose を実行
OP       := op run --env-file=$(ENV_FILE) --
# compose ファイルの必須変数(${DEEPSEEK_API_KEY:?}, ${SEARCH_API_KEY:?})を満たすためのダミー（値は不問の操作用）
# CLAUDE_API_KEY は現状 compose 側で任意だが、切り戻し時の必須化にも備えてダミーを同梱する。
DUMMY    := DEEPSEEK_API_KEY=dummy SEARCH_API_KEY=dummy CLAUDE_API_KEY=dummy

BACKEND_URL  := http://localhost:8080
FRONTEND_URL := http://localhost:8081

.DEFAULT_GOAL := help

.PHONY: help
help: ## このヘルプを表示
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| sort \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ---- 起動系（1Password から秘密を注入）----

.PHONY: up
up: ## 全サービス起動（op 注入）
	$(OP) $(COMPOSE) up -d

.PHONY: up-build
up-build: ## 再ビルドして全サービス起動（op 注入）
	$(OP) $(COMPOSE) up -d --build

.PHONY: restart
restart: ## 全サービス再作成（op 注入）
	$(OP) $(COMPOSE) up -d --force-recreate

.PHONY: restart-backend
restart-backend: ## backend のみ再作成（op 注入）
	$(OP) $(COMPOSE) up -d --force-recreate backend

.PHONY: rebuild-backend
rebuild-backend: ## backend を再ビルドして再作成（op 注入）
	$(OP) $(COMPOSE) up -d --build backend

.PHONY: rebuild-frontend
rebuild-frontend: ## frontend を再ビルドして再作成（op 注入）
	$(OP) $(COMPOSE) up -d --build frontend

# ---- 停止・確認系（秘密不要：ダミーキーで即実行）----

.PHONY: down
down: ## 全サービス停止・削除
	$(DUMMY) $(COMPOSE) down

.PHONY: down-v
down-v: ## 全サービス停止・削除＋ボリューム削除（DB初期化）
	$(DUMMY) $(COMPOSE) down -v

.PHONY: stop
stop: ## 全サービス停止（コンテナは残す）
	$(DUMMY) $(COMPOSE) stop

.PHONY: ps
ps: ## サービス一覧
	$(DUMMY) $(COMPOSE) ps

.PHONY: logs
logs: ## 全ログ追従
	$(DUMMY) $(COMPOSE) logs -f

.PHONY: logs-backend
logs-backend: ## backend ログ追従
	$(DUMMY) $(COMPOSE) logs -f backend

.PHONY: logs-mysql
logs-mysql: ## mysql ログ追従
	$(DUMMY) $(COMPOSE) logs -f mysql

# ---- デバッグ補助 ----

.PHONY: check-op
check-op: ## 1Password 参照が解決できるか確認（秘密はマスク）
	@echo "LLM_PROVIDER     = $$(op read 'op://Development/CalendarRabbit/LLM_PROVIDER')"
	@echo "DEEPSEEK_MODEL   = $$(op read 'op://Development/CalendarRabbit/DEEPSEEK_MODEL')"
	@echo "SEARCH_PROVIDER  = $$(op read 'op://Development/CalendarRabbit/SEARCH_PROVIDER')"
	@echo "DB_USER          = $$(op read 'op://Development/CalendarRabbit/DB_USER')"
	@echo "DB_NAME          = $$(op read 'op://Development/CalendarRabbit/DB_NAME')"
	@echo "DEEPSEEK_API_KEY = $$(op read 'op://Development/CalendarRabbit/DEEPSEEK_API_KEY' | sed 's/./*/g')"
	@echo "SEARCH_API_KEY   = $$(op read 'op://Development/CalendarRabbit/SEARCH_API_KEY' | sed 's/./*/g')"
	@echo "CLAUDE_API_KEY   = $$(op read 'op://Development/CalendarRabbit/CLAUDE_API_KEY' | sed 's/./*/g')"
	@echo "GOOGLE_OAUTH_CLIENT_ID     = $$(op read 'op://Development/CalendarRabbit/GOOGLE_OAUTH_CLIENT_ID')"
	@echo "GOOGLE_OAUTH_REDIRECT_URL  = $$(op read 'op://Development/CalendarRabbit/GOOGLE_OAUTH_REDIRECT_URL')"
	@echo "GOOGLE_OAUTH_CLIENT_SECRET = $$(op read 'op://Development/CalendarRabbit/GOOGLE_OAUTH_CLIENT_SECRET' | sed 's/./*/g')"
	@echo "GOOGLE_TOKEN_ENC_KEY       = $$(op read 'op://Development/CalendarRabbit/GOOGLE_TOKEN_ENC_KEY' | sed 's/./*/g')"

.PHONY: mysql
mysql: ## MySQL に接続（コンテナ内 mysql クライアント）
	$(DUMMY) $(COMPOSE) exec mysql sh -c 'mysql -ucalendarrabbit -pcalendarrabbit calendarrabbit'

.PHONY: sh-backend
sh-backend: ## backend コンテナでシェル（distroless のため sh が無い場合あり）
	$(DUMMY) $(COMPOSE) exec backend sh || echo "distroless イメージには shell がありません。logs-backend を利用してください。"

.PHONY: health
health: ## backend/フロント経由のヘルスチェック
	@echo "backend : $$(curl -s -o /dev/null -w '%{http_code}' $(BACKEND_URL)/api/health)"
	@echo "proxy   : $$(curl -s -o /dev/null -w '%{http_code}' $(FRONTEND_URL)/api/health)"

.PHONY: swagger
swagger: ## Swagger UI の URL を表示
	@echo "Swagger UI: $(BACKEND_URL)/swagger/index.html"

.PHONY: e2e
e2e: ## E2Eデバッグ: チャット送信→予定案IDを表示（承認は approve PROPOSAL=<id>）
	@echo ">>> POST /api/chat/messages (実 LLM 呼び出し)"; \
	curl -s --max-time 180 -X POST $(BACKEND_URL)/api/chat/messages \
		-H 'Content-Type: application/json' \
		-d '{"content":"TGSの予定を追加して"}' | python3 -m json.tool

.PHONY: approve
approve: ## 予定案を承認: make approve PROPOSAL=1
	@test -n "$(PROPOSAL)" || (echo "usage: make approve PROPOSAL=<id>"; exit 1)
	curl -s -X POST $(BACKEND_URL)/api/chat/proposals/$(PROPOSAL)/approve | python3 -m json.tool

.PHONY: events
events: ## 当月イベント一覧を表示
	@curl -s "$(BACKEND_URL)/api/calendar/events" | python3 -m json.tool

# ---- テスト（参考）----

.PHONY: test
test: ## backend/frontend の全テスト実行
	cd backend && go test ./...
	cd frontend && pnpm test
