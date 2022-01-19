# Syntect Server

This is an HTTP server that exposes the Rust [Syntect](https://github.com/trishume/syntect) syntax highlighting library for use by other services. Send it some code, and it'll send you syntax-highlighted code in response.

## Usage

```bash
docker run --detach --name=syntax-highlighter -p 9238:9238 sourcegraph/syntax-highlighter
```

You can then e.g. `GET` http://localhost:9238/health or http://host.docker.internal:9238/health to confirm it is working.

## API

See [API](./docs/api.md)

## Client

[gosyntect](https://github.com/sourcegraph/gosyntect) is a Go package + CLI program to make requests against syntect_server.

## Configuration

By default on startup, `syntect_server` will list all features (themes + file types) it supports. This can be disabled by setting `QUIET=true` in the environment.

## Development

1. [Install Rust **nightly**](https://rocket.rs/guide/getting-started/#installing-rust).
2. `git clone` this repository anywhere on your filesystem.
3. Use `cargo run` to download dependencies + compile + run the server.
4. Use `cargo build --release` to create release binaries (see `./target/release/`)


## Building docker image

`./build.sh` will build your current repository checkout into a final Docker image.

**NOTE**: The docker image will be published automatically via CI.

## Updating Sourcegraph

Once published, the image version will need to be updated in the following locations to make Sourcegraph use it:

- [`sourcegraph/sourcegraph > cmd/server/Dockerfile`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/cmd/server/Dockerfile?subtree=true#L54:13)
- [`sourcegraph/sourcegraph > sg.config.yaml`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/sg.config.yaml?subtree=true#L206:7)

Additionally, it's worth doing a [search](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+sourcegraph/syntect_server:&patternType=literal) for other uses in case this list is stale.

## Code hygiene

- Use `cargo fmt` or an editor extension to format code.

## Adding themes

- Copy a `.tmTheme` file anywhere under `./syntect/testdata` (make a new dir if needed) [in our fork](https://github.com/slimsag/syntect).
- `cd syntect && make assets`
- In this repo, `cargo update -p syntect`.
- Build a new binary.

## Adding languages:

TODO: Update these instructions once we move to sourcegraph/syntect.

#### 1) Find an open-source `.tmLanguage` or `.sublime-syntax` file and send a PR to our package registry

https://github.com/slimsag/Packages is the package registry we use which holds all of the syntax definitions we use in syntect_server and Sourcegraph. Send a PR there by following [these steps](https://github.com/slimsag/Packages/blob/master/README.md#adding-a-new-language)

#### 2) Update our temporary fork of `syntect`

We use a temporary fork of `syntect` as a hack to get our `Packages` registry into the binary. Update it by creating a PR with two commits like:

- https://github.com/slimsag/syntect/commit/9976d2095e49fd91607026364466cd7b389b938e
- https://github.com/slimsag/syntect/commit/1182dd3bd7c82b6655d8466c9896a1e4f458c71e

#### 3) Update syntect_server to use the new version of `syntect`

Run the following in this directory.

```
$ cargo update -p syntect
```

## Supported languages:

Run: `cargo run --bin syntect_server` to see supported languages and themes.
