// Atlas 設定。ent スキーマをソースにして goose 形式のバージョン管理マイグレーションを扱う。
// マイグレーション生成は `go run -mod=mod ./ent/migrate/main.go <name>` を使用する
// （dev データベースは ATLAS_DEV_URL で指定）。
data "external_schema" "ent" {
  program = [
    "go", "run", "-mod=mod",
    "ariga.io/atlas-provider-ent",
    "--path", "./ent/schema",
    "--dialect", "mysql",
  ]
}

env "local" {
  src = data.external_schema.ent.url
  dev = "docker://mysql/8/dev"
  migration {
    dir    = "file://db/migrations"
    format = goose
  }
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}
