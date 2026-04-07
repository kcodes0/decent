# Things You Could Build

This document is half roadmap and half invitation.

Some of these ideas are practical things we expect to build around `decent`.
Some are more speculative. All of them fit the basic idea: a cooperative network
for serving static sites from many independently run nodes.

## Practical things to build soon

Implemented in `v0.0.3`

### ~~Operator checks~~

~~A built-in operator check tool that validates local config, manifest values,
daemon reachability, main API reachability, and common reverse-proxy mistakes
before a node joins the network.~~

### 1. A real dashboard

The protocol already has enough structure for a decent control panel.

You could build:

- a web UI for the main node
- a worker health table
- region and latency views
- content hash mismatch alerts
- a live map of active nodes

### 2. A one-command main-node deployer

Right now the happy path assumes you already know how to run a daemon behind a
reverse proxy.

You could build:

- a `decent deploy` command
- systemd service generation
- Caddy or nginx templates
- TLS setup helpers
- reverse-proxy config helpers for path-prefix APIs like `/decent`

### 3. Worker onboarding pages

A site could publish a “host this site” page that gives volunteers everything they
need to join.

You could build:

- a worker invite page
- region suggestions
- copy-paste setup commands
- node requirements
- a volunteer reputation layer

### 4. Better routing

Today routing is intentionally simple.

You could build:

- GeoIP-based redirects
- latency-aware selection using active probes
- sticky routing
- weighted balancing
- canary nodes for new site versions

### 5. Persistent registry storage

The current registry is in memory.

You could build:

- SQLite-backed node state
- history of node uptime
- failure audit trails
- trust scores
- operator notes and labels

### 6. Safer integrity checks

The current trust model is practical but soft.

You could build:

- signed manifests
- signed heartbeats
- transparency logs
- reproducible build attestations
- worker proofs that the checked-out tree matches a signed release

## Product ideas people could build on top

### 7. Community mirrors for indie sites

This is the obvious first use case.

Imagine:

- personal blogs with volunteer mirrors
- artist portfolios with community-run edge nodes
- nonprofit sites that can survive traffic spikes
- event pages that keep working when one host goes down

### 8. A federation for open documentation

Docs sites are usually static and globally read.

You could build:

- decentralized docs hosting
- open source project mirrors
- per-region volunteer docs nodes
- offline-friendly regional documentation hubs

### 9. Civic or emergency publishing networks

Static sites are a good fit for resilient public information.

You could build:

- local emergency information mirrors
- election information mirrors
- volunteer-run municipal notice networks
- disaster-response static publishing clusters

### 10. Educational hosting swarms

This protocol could be a teaching tool as much as a hosting tool.

You could build:

- classroom-run regional nodes
- distributed systems labs
- networking demos
- protocol experiments around trust and discovery

## Developer tools people could add

### 11. A protocol SDK

You could build SDKs in:

- TypeScript
- Rust
- Go
- Python

That would make it easier to build third-party controllers, dashboards, and node tools.

### 12. A testing and simulation lab

This one would be especially useful for the protocol itself.

You could build:

- a local cluster simulator
- network partition testing
- bad-node simulations
- latency injection
- fake regional traffic generators

### 13. A site-builder adapter layer

Most static sites are built with other tools first.

You could build adapters for:

- Astro
- Next.js static export
- Vite
- Eleventy
- Hugo
- Jekyll

That could make `decent push` smarter about build output and publish steps.

## Weird but interesting ideas

### 14. User-chosen routing

Visitors do not always want the “nearest” node.

You could build:

- privacy-first routing
- low-carbon routing
- community-trusted routing
- “always use this region” preferences

### 15. Signed archival snapshots

You could treat every published hash as a durable public snapshot.

You could build:

- immutable historical mirrors
- snapshot browsers
- rollback explorers
- public release timelines for static sites

### 16. Shared cultural archives

A protocol like this could host materials that communities want mirrored widely.

You could build:

- digital zines
- museum exhibit mirrors
- oral history sites
- language preservation sites

## Bigger protocol directions

### 17. Multi-main federation

Right now one site has one main node.

You could explore:

- multiple authoritative main nodes
- quorum-based manifest updates
- cross-signed site state
- regional failover mains

### 18. Better discovery

The current discovery layer is direct and simple by design.

You could explore:

- GeoDNS
- gossip-based discovery
- signed node registries
- public node indexes
- private invite-only federations

### 19. Incentive layers

The protocol does not need tokens to be useful, but people will still want
accountability and reward systems.

You could build:

- reputation systems
- public contribution history
- badges or trust levels
- donation links for operators
- cooperative credit systems
