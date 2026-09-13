# syntax=docker/dockerfile:1

FROM node:24-alpine AS build
WORKDIR /app
RUN corepack enable
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
# esbuild はビルドスクリプトを持つ信頼済み依存のため、ビルドを許可する。
RUN pnpm install --frozen-lockfile --config.dangerouslyAllowAllBuilds=true
COPY . .
RUN pnpm build

FROM nginx:1.27-alpine
ENV BACKEND_URL=http://backend:8080
COPY nginx.conf.template /etc/nginx/templates/default.conf.template
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
