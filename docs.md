# decent docs

`decent` is an early alpha federated hosting tool for static sites. This build is
version `v0.0.1`.

The short version:

- one machine is the main node
- that machine owns the git repo and publishes `decent.toml`
- other people can volunteer machines as worker nodes
- workers clone the repo, verify the files, serve the site, and report health
- the main node redirects visitors to a healthy nearby worker when it can
- if no worker is a good fit, the main node serves the site itself

## What lives where

There are two binaries:

- `decent`
  The CLI you run by hand.
- `decent-node`
  The long-running daemon that actually serves traffic.

There are also two important files:

- `decent.toml`
  Stored in the site repo. This is the shared manifest for the network.
- `~/.config/decent/node.toml`
  Stored on each machine. This is that machine's local node config.

## Install

If the repo is published on GitHub, the intended install flow is:

```sh
curl -fsSL https://raw.githubusercontent.com/kcodes0/decent/main/install.sh | sh
```

That script:

- downloads the source
- builds `decent` and `decent-node`
- installs them into `~/.local/bin`
- adds `~/.local/bin` to your shell PATH if needed

After that, check the install:

```sh
decent version
decent --help
```

## Main concepts

### Main node

The main node is the source of truth for the site.

It does three jobs:

- owns the git repo
- tracks which worker nodes are healthy
- acts as the fallback origin when no worker should be used

### Worker node

A worker node is a volunteer machine that helps serve a static site.

It does three jobs:

- clones and syncs the repo
- verifies its local files match the expected content hash
- serves the site and sends heartbeats to the main node

### Manifest

The manifest is the repo-level contract for the network.

Today it includes:

- site name
- repo location
- main node region
- main node public site URL
- main node API URL
- current content hash
- known nodes

### Integrity model

This project is cooperative, not adversarially hardened.

The trust model is:

- the git repo is the canonical source of site content
- the manifest stores the expected content hash
- workers re-hash local files after sync
- if a worker drifts, it resets back to the repo state
- if a worker keeps reporting bad hashes or failing heartbeats, the main node can stop trusting it

This is good enough for an alpha. It is not meant to be strong cryptographic proof.

## Commands

### `decent init`

Run this in the main site repo.

It walks you through:

- site name
- GitHub repo
- main node region label
- public site URL
- main API URL

Then it:

- creates `decent.toml`
- saves local node config
- optionally commits the manifest

After `decent init`, start the main daemon:

```sh
decent-node --config ~/.config/decent/node.toml
```

### `decent setup`

Run this on a worker machine.

It walks you through:

- region label
- host/IP
- public port
- admin port
- main API URL
- main site URL
- storage and bandwidth budget
- sync interval
- heartbeat interval

Then it writes the local worker config.

### `decent host <repo>`

Run this on a worker machine after setup.

Examples:

```sh
decent host github:kcodes0/decent-site
decent host https://github.com/kcodes0/decent-site.git
```

It will:

- clone the repo
- load `decent.toml`
- verify the site hash
- save worker config
- start `decent-node`

### `decent status`

Shows:

- local node info
- local daemon status if reachable
- network status from the main node if reachable

### `decent push`

Run this on the main repo after changing site content.

It will:

- re-hash the site contents
- update `decent.toml`
- refresh the known node list from the main API when available
- commit the manifest update
- push the repo

## HTTP protocol

The current protocol is simple HTTP with JSON bodies.

### `POST /api/register`

Workers call this when joining.

It sends:

- worker ID
- role
- region
- public URL
- admin URL
- capacity details
- current content hash

### `POST /api/heartbeat`

Workers call this on a loop.

It sends:

- node ID
- health status
- uptime
- current content hash
- latency
- capacity details

### `GET /api/status`

Returns:

- main node info
- manifest
- known nodes
- healthy nodes

### `GET /api/route`

Returns the current routing choice for a request.

## Routing model

`decent` currently uses redirect-based routing instead of GeoDNS.

That choice is deliberate for `v0.0.1`:

- it is much easier to self-host
- it works without third-party CDN services
- the main node always remains the fallback
- it can evolve into a GeoDNS setup later

The selector currently uses:

- explicit region labels
- request hints
- node health
- node latency
- recent heartbeat freshness

## Typical flow

### Main node flow

1. Put your static site in a git repo.
2. Run `decent init`.
3. Start `decent-node`.
4. When you update the site, run `decent push`.

### Worker flow

1. Install `decent`.
2. Run `decent setup`.
3. Run `decent host <repo>`.
4. Leave `decent-node` running.

## Current limits

This project is intentionally small and early.

Right now it is:

- static files only
- single-binary daemon, with main and worker roles
- redirect-based routing
- git plus content hashing for integrity
- focused on the happy path

It does not yet cover:

- TLS automation
- persistent registry storage
- cryptographic signing
- multi-repo orchestration
- production-grade observability
- DDoS mitigation

## Release

Current release: `v0.0.1`
