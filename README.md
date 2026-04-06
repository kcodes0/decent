# decent

`decent` is a federated static-site hosting prototype.

One machine acts as the master node. It owns the git repo, publishes a `decent.toml`
manifest, tracks healthy worker nodes, and serves as the fallback origin.

Volunteer worker nodes clone the repo, verify the checked-out files against the
manifest content hash, self-correct if local files diverge, serve the static site,
and heartbeat back to the master.

## Binaries

- `decent`: CLI for master and worker setup
- `decent-node`: long-running daemon for either `master` or `worker` mode

## Protocol

The current protocol is HTTP + TOML:

- `POST /api/register`
  Worker registration payload with region, capacity, URLs, and current content hash.
- `POST /api/heartbeat`
  Worker health report with uptime, latency, and content hash.
- `GET /api/status`
  Master registry snapshot, manifest, and healthy node list.
- `GET /api/route`
  Routing decision for a visitor region hint.

The master routes normal site traffic by HTTP redirect. If no healthy worker is a
better fit, the master serves the site directly from its own filesystem.

## Happy Path

1. On the master repo, run `decent init`.
2. Start the master daemon with `decent-node --config ~/.config/decent/node.toml`.
3. On a worker, run `decent setup`.
4. On that worker, run `decent host github:user/repo`.
5. Publish updates from the master repo with `decent push`.

## Current Routing Choice

The prototype uses the hybrid-friendly redirect layer first, not GeoDNS.

That keeps the implementation self-hostable and simple:

- the master always has a local fallback
- workers can join without DNS control
- GeoDNS can be added later behind the same registry and selection logic

Region matching currently uses explicit region tags and request hints, which is good
enough for the cooperative-network prototype. A production upgrade path would be to
add a local GeoIP database to the master router.
