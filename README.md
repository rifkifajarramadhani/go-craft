# go-craft

`go-craft` is an offline, composable generator for clean-architecture Go services. It embeds a versioned starter and emits only the selected HTTP, database, queue, search, and scheduler adapters.

## Install

```sh
go install github.com/rifkifajarramadhani/go-craft/cmd/go-craft@latest
```

## Create a project

Run the interactive wizard:

```sh
go-craft new my-service
```

Or specify the complete selection non-interactively:

```sh
go-craft new my-service \
  --module=github.com/acme/my-service \
  --framework=gin \
  --database=postgres \
  --queue=redis \
  --search=elasticsearch \
  --scheduler \
  --no-interaction
```

Choices are `nethttp|fiber|gin|echo`, `mysql|postgres|sqlite`, `none|database|redis` for queues, and `none|elasticsearch` for search. Fiber, MySQL, and the database queue are the defaults; search defaults to none. A queue adds `worker`, `queue`, and `all`; `--scheduler` additionally adds `scheduler` and `schedule`.

The module path resolves from `--module`, then `--owner`, then the saved GitHub owner:

```sh
go-craft config set github-owner acme
go-craft config get github-owner
```

Shell completions and version information are available through `go-craft completion <shell>` and `go-craft version`.

Generation happens in a staging directory. Source is formatted before the destination becomes visible, then `go mod tidy` runs in the finished project. If tidy fails, the project remains intact and the CLI prints the recovery command.

## Template maintenance

In the starter repository, `make template-pack` produces a deterministic, versioned tarball and SHA-256 file. In this repository, `make update-template` verifies that checksum before replacing the embedded snapshot. `go-craft new` performs no template downloads.

### Elasticsearch 9.x migration

Migrate the search adapter before Elasticsearch 8.x maintenance ends on
January 15, 2027. [Elastic lifecycle dates](https://www.elastic.co/support/eol)

1. Bring a staging cluster to the latest 8.19 patch. Take a snapshot and
   verify that it restores to an 8.19 cluster. Resolve Upgrade Assistant
   blockers, including any indices that need reindexing.
   [Elastic upgrade preparation](https://www.elastic.co/docs/deploy-manage/upgrade/prepare-to-upgrade)
2. Test the 8.x Go client against a supported 9.x server in REST compatibility
   mode. Run the adapter integration tests before and after switching to the
   9.x Go client.
   [Elastic REST compatibility](https://www.elastic.co/docs/reference/elasticsearch/rest-apis/compatibility)
3. Update the generated Compose image, the Go client, and the CI integration
   image together. Document the upgrade steps for existing generated projects.

## Verification

```sh
make check
```

The scaffold test generates and structurally validates all 120 valid combinations. CI also compiles a pairwise matrix on every change and the full matrix nightly.
