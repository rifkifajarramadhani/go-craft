package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func configureProjectFiles(destination string, options Options) error {
	files := map[string]string{
		".env.example":        environmentExample(options),
		"Makefile":            makefile(options),
		"Dockerfile":          dockerfile(options),
		"docker-compose.yml":  compose(options),
		"configs/config.yaml": configuration(options),
		"README.md":           readme(options),
	}
	for relative, content := range files {
		path := filepath.Join(destination, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create directory for %s: %w", relative, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write generated %s: %w", relative, err)
		}
	}

	return nil
}

func environmentExample(options Options) string {
	value := `# Copy this file to .env for local development. Never commit real credentials.
APP_ENVIRONMENT=development

DATABASE_PASSWORD=local-development-only
AUTH_JWT_ACCESS_SECRET=local-access-secret-change-me
AUTH_JWT_REFRESH_SECRET=local-refresh-secret-change-me
REDIS_PASSWORD=
MAIL_PASSWORD=
`
	if options.Search == "elasticsearch" {
		value += `ELASTICSEARCH_URL=http://elasticsearch:9200
ELASTICSEARCH_USERNAME=
ELASTICSEARCH_PASSWORD=
`
	}

	return value
}

func makefile(options Options) string {
	databaseURL := "mysql://$(DATABASE_USER):$(DATABASE_PASSWORD)@tcp(db:3306)/$(DATABASE_NAME)"
	migrationTag := "mysql"
	switch options.Database {
	case "postgres":
		databaseURL = "postgres://$(DATABASE_USER):$(DATABASE_PASSWORD)@db:5432/$(DATABASE_NAME)?sslmode=disable"
		migrationTag = "postgres"
	case "sqlite":
		databaseURL = "sqlite://$(DATABASE_NAME)"
		migrationTag = "sqlite"
	}

	roles := "\nrun-server:\n\tgo run ./cmd/app serve\n"
	if options.Queue != "none" {
		roles += "\nrun-worker:\n\tgo run ./cmd/app worker\n\nrun-all:\n\tgo run ./cmd/app all\n\nqueue:\n\tgo run ./cmd/app queue $(args)\n"
	}
	if options.Scheduler {
		roles += "\nrun-scheduler:\n\tgo run ./cmd/app scheduler\n\nschedule:\n\tgo run ./cmd/app schedule $(args)\n"
	}

	return fmt.Sprintf(`-include .env
export

DATABASE_USER ?= root
DATABASE_NAME ?= db_name
export DATABASE_URL ?= %s

MIGRATIONS := internal/adapter/%s/migrations
MIGRATE_TAG := %s

.PHONY: build migrate-create migrate fmt fmt-check vet lint test test-race vuln check

build:
	go build ./cmd/app
%s
migrate-create:
	docker compose exec server migrate create -ext sql -dir $(MIGRATIONS) -seq $(name)

migrate:
	docker compose exec server migrate -database '$(DATABASE_URL)' -path $(MIGRATIONS) $(args)

fmt:
	goimports -w .
	gofmt -w .

fmt-check:
	test -z "$$(gofmt -l .)"
	test -z "$$(goimports -l .)"

vet:
	go vet ./...

lint:
	golangci-lint run

test:
	go test ./...

test-race:
	go test -race ./...

vuln:
	govulncheck ./...

check: fmt-check vet lint test test-race
`, databaseURL, options.Database, migrationTag, roles)
}

func dockerfile(options Options) string {
	migrationTag := options.Database
	if migrationTag == "sqlite" {
		migrationTag = "sqlite"
	}

	return fmt.Sprintf(`FROM golang:1.27.4-alpine3.24 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN mkdir -p /out && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/app

FROM build AS development
RUN go install github.com/air-verse/air@v1.65.3 && \
    go install -tags %s github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1
WORKDIR /app
COPY . .
CMD ["air", "-c", ".air.toml"]

FROM alpine:3.22 AS production
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S app && adduser -S -G app app && \
    mkdir -p /app/data && chown -R app:app /app
WORKDIR /app
COPY --from=build /out/ /usr/local/bin/
COPY --chown=app:app configs ./configs
USER app
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/health/live || exit 1
CMD ["app", "serve"]
`, migrationTag)
}

func configuration(options Options) string {
	database := "  host: db\n  port: 3306\n  user: root\n  password: \"\"\n  name: db_name"
	switch options.Database {
	case "postgres":
		database = "  host: db\n  port: 5432\n  user: postgres\n  password: \"\"\n  name: db_name"
	case "sqlite":
		database = "  host: local\n  port: 1\n  user: sqlite\n  password: \"\"\n  name: data/app.db"
	}

	redis := ""
	if options.Queue == "redis" {
		redis = `
redis:
  address: redis:6379
  password: ""
  db: 0
`
	}
	queue := "queue:\n  driver: none"
	if options.Queue != "none" {
		queue = fmt.Sprintf(`queue:
  driver: %s
  concurrency: 10
  shutdown_seconds: 30
  queues:
    default: 3
    mail: 2
    maintenance: 1`, options.Queue)
		if options.Queue == "database" {
			queue += `
  database:
    poll_interval_milliseconds: 500
    reservation_seconds: 60`
		}
	}
	scheduler := ""
	if options.Scheduler {
		scheduler = "\nscheduler:\n  timezone: UTC\n"
	}
	elasticsearch := ""
	if options.Search == "elasticsearch" {
		elasticsearch = `
elasticsearch:
  url: http://elasticsearch:9200
  username: ""
  password: ""
`
	}

	return fmt.Sprintf(`app:
  port: 8080
  environment: development

database:
%s

%s
auth:
  jwt_access_secret: ""
  jwt_refresh_secret: ""
  access_ttl_minutes: 15
  refresh_ttl_hours: 168
  issuer: %s
  audience: %s-api
  verification_ttl_hours: 24
  bootstrap_admin_email: ""
%s
%s
%s

mail:
  host: mailpit
  port: 1025
  username: ""
  password: ""
  encryption: none
  from_address: hello@example.com
  from_name: %s

logging:
  level: info
  file: logs/app.log
`, database, elasticsearch, options.Name, options.Name, redis, queue, scheduler, options.Name)
}

func compose(options Options) string {
	depends := "      db:\n        condition: service_healthy\n"
	environment := "      DATABASE_HOST: db\n"
	var databaseService string

	switch options.Database {
	case "mysql":
		databaseService = mysqlService
	case "postgres":
		databaseService = postgresService
	case "sqlite":
		depends = ""
		environment = "      DATABASE_NAME: data/app.db\n"
	}

	redisDependency := ""
	redisService := ""
	if options.Queue == "redis" {
		redisDependency = "      redis:\n        condition: service_healthy\n"
		redisService = redisComposeService
		environment += "      REDIS_ADDRESS: redis:6379\n"
	}

	serverEnvironment := environment
	elasticsearchDependency := ""
	elasticsearchService := ""
	if options.Search == "elasticsearch" {
		elasticsearchDependency = "      elasticsearch:\n        condition: service_healthy\n"
		elasticsearchService = elasticsearchComposeService
		serverEnvironment += "      ELASTICSEARCH_URL: http://elasticsearch:9200\n"
	}

	worker := ""
	if options.Queue != "none" {
		worker = fmt.Sprintf(workerService, options.Queue, environment, depends+redisDependency)
	}
	scheduler := ""
	if options.Scheduler {
		scheduler = fmt.Sprintf(schedulerService, options.Queue, environment, composeDependencies(depends+redisDependency))
	}

	// go_build_cache/go_mod_cache are shared by the Go dev services so rebuilds
	// reuse one compile cache (the Go cache is concurrency-safe) and it
	// survives container recreates.
	volumes := "volumes:\n  go_build_cache:\n  go_mod_cache:\n"
	if options.Database != "sqlite" {
		volumes += "  db_data:\n"
	}
	if options.Search == "elasticsearch" {
		volumes += "  elasticsearch_data:\n"
	}

	return fmt.Sprintf(`%s
services:
  server:
    build:
      context: .
      dockerfile: Dockerfile
      target: development
    command: air -c .air.toml
    ports:
      - "8080:8080"
    volumes:
      - .:/app
      - go_build_cache:/root/.cache/go-build
      - go_mod_cache:/go/pkg/mod
    env_file:
      - ${ENV_FILE:-.env}
    environment:
%s    depends_on:
%s      mailpit:
        condition: service_healthy

%s%s%s%s%s  mailpit:
    image: axllent/mailpit:v1.27.8
    ports:
      - "1025:1025"
      - "8025:8025"
    healthcheck:
      test: ["CMD", "/mailpit", "readyz"]
      interval: 5s
      timeout: 3s
      retries: 20
`, volumes, serverEnvironment, depends+redisDependency+elasticsearchDependency, worker, scheduler, databaseService, redisService, elasticsearchService)
}

func composeDependencies(services string) string {
	if services == "" {
		return ""
	}

	return "    depends_on:\n" + services
}

func readme(options Options) string {
	components := []string{
		"HTTP: `" + options.Framework + "`", "database: `" + options.Database + "`",
		"queue: `" + options.Queue + "`", "search: `" + options.Search + "`",
	}
	if options.Scheduler {
		components = append(components, "scheduler: enabled")
	}

	commands := "go run ./cmd/app serve"
	if options.Queue != "none" {
		commands += "\ngo run ./cmd/app worker\ngo run ./cmd/app all"
	}
	if options.Scheduler {
		commands += "\ngo run ./cmd/app scheduler"
	}
	searchDocumentation := ""
	if options.Search == "elasticsearch" {
		searchDocumentation = "\n## Elasticsearch\n\n" +
			"The HTTP process verifies Elasticsearch at startup and includes it in `/health/ready`. " +
			"The local Compose service disables authentication and TLS for development only. Configure " +
			"`ELASTICSEARCH_URL`, `ELASTICSEARCH_USERNAME`, and `ELASTICSEARCH_PASSWORD` for secured remote deployments.\n"
	}

	return fmt.Sprintf(`# %s

Generated by go-craft with %s.

## Start

~~~sh
cp .env.example .env
docker compose up -d
make migrate
%s
~~~

The project keeps domain and application logic independent from transport and persistence adapters. The generated source, dependencies, configuration, containers, and commands contain only the selected adapters.
%s`, options.Name, strings.Join(components, ", "), commands, searchDocumentation)
}

const mysqlService = `  db:
    image: mysql:8.4
    environment:
      MYSQL_ROOT_PASSWORD: ${DATABASE_PASSWORD:?DATABASE_PASSWORD must be set in .env}
      MYSQL_DATABASE: db_name
    volumes:
      - db_data:/var/lib/mysql
    healthcheck:
      test: ["CMD-SHELL", "mysqladmin ping -h localhost -u root -p$$MYSQL_ROOT_PASSWORD"]
      interval: 5s
      timeout: 5s
      retries: 20

`

const postgresService = `  db:
    image: postgres:17-alpine
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: ${DATABASE_PASSWORD:?DATABASE_PASSWORD must be set in .env}
      POSTGRES_DB: db_name
    volumes:
      - db_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres -d db_name"]
      interval: 5s
      timeout: 5s
      retries: 20

`

const redisComposeService = `  redis:
    image: redis:7.4-alpine
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 20

`

const elasticsearchComposeService = `  elasticsearch:
    image: docker.elastic.co/elasticsearch/elasticsearch:8.19.25
    ports:
      - "9200:9200"
    environment:
      discovery.type: single-node
      xpack.security.enabled: "false"
      xpack.security.http.ssl.enabled: "false"
    volumes:
      - elasticsearch_data:/usr/share/elasticsearch/data
    healthcheck:
      test: ["CMD-SHELL", "curl -fsS 'http://localhost:9200/_cluster/health?wait_for_status=yellow&timeout=5s' >/dev/null"]
      interval: 5s
      timeout: 5s
      retries: 30

`

const workerService = `  worker:
    build:
      context: .
      dockerfile: Dockerfile
      target: development
    command: air -c .air.worker.toml
    volumes:
      - .:/app
      - go_build_cache:/root/.cache/go-build
      - go_mod_cache:/go/pkg/mod
    env_file:
      - ${ENV_FILE:-.env}
    environment:
      QUEUE_DRIVER: %s
%s    depends_on:
%s      mailpit:
        condition: service_healthy

`

const schedulerService = `  scheduler:
    build:
      context: .
      dockerfile: Dockerfile
      target: development
    command: go run ./cmd/app scheduler
    volumes:
      - .:/app
      - go_build_cache:/root/.cache/go-build
      - go_mod_cache:/go/pkg/mod
    env_file:
      - ${ENV_FILE:-.env}
    environment:
      QUEUE_DRIVER: %s
%s%s
`
