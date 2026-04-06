# decent

`decent` is an early alpha federated hosting tool for static sites.

Current version: `v0.0.1`

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/kcodes0/decent/main/install.sh | sh
```

Then verify it:

```sh
decent version
decent --help
```

## What it does

- the main node owns the git repo and serves as the fallback origin
- worker nodes clone the repo, verify it, serve it, and report health
- visitors get redirected to a healthy nearby worker when one is available
- the main node serves the site directly when no worker is a good fit

## Quick start

1. In your site repo, run `decent init`.
2. Start the main daemon with `decent-node --config ~/.config/decent/node.toml`.
3. On a worker machine, run `decent setup`.
4. On that worker machine, run `decent host <repo>`.
5. When you publish updates, run `decent push` from the main repo.

## Docs

See [docs.md](/Users/jason/Code/decent/docs.md) for the protocol, architecture, setup flow, and current limits.
