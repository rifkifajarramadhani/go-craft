# go-craft

`go-craft` is an offline, composable generator for clean-architecture Go services. It embeds a versioned starter and emits only the selected HTTP, database, queue, and scheduler adapters.

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
  --scheduler \
  --no-interaction
```

Choices are `nethttp|fiber|gin|echo`, `mysql|postgres|sqlite`, and `none|database|redis`. Fiber, MySQL, and the database queue are the defaults. A queue adds `worker`, `queue`, and `all`; `--scheduler` additionally adds `scheduler` and `schedule`.

The module path resolves from `--module`, then `--owner`, then the saved GitHub owner:

```sh
go-craft config set github-owner acme
go-craft config get github-owner
```

Shell completions and version information are available through `go-craft completion <shell>` and `go-craft version`.

Generation happens in a staging directory. Source is formatted before the destination becomes visible, then `go mod tidy` runs in the finished project. If tidy fails, the project remains intact and the CLI prints the recovery command.

## Template maintenance

In the starter repository, `make template-pack` produces a deterministic, versioned tarball and SHA-256 file. In this repository, `make update-template` verifies that checksum before replacing the embedded snapshot. `go-craft new` performs no template downloads.

## Verification

```sh
make check
```

The scaffold test generates and structurally validates all 60 valid combinations. CI also compiles a pairwise matrix on every change and the full matrix nightly.

