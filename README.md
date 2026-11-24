# starTeam Control Plane

A Go gateway for local application components. It probes a primary and standby backend, forwards read requests to a ready service, and exposes backend readiness through a local status endpoint.

## Watch a failover

From a checkout with Go 1.25 or newer:

```sh
go run ./cmd/starteam-control demo
```

The demo starts two temporary loopback services. The first status reports `active: primary`. After a simulated failure, the second reports `active: standby`. Run `go test ./...` to check routing, failover, and recovery.

## Connect your services

Copy [the example configuration](examples/config.json) and set the two origins to services that implement `/healthz`.

```sh
go run ./cmd/starteam-control --config examples/config.json
curl http://127.0.0.1:8765/_control/status
```

The gateway listens on loopback and checks readiness for each read request. On macOS, build the binary and use `write-plist` with `--root`, an absolute `--binary` path, and an absolute `--config` path to render a LaunchAgent into your runtime directory.

The [MIT-licensed release](LICENSE) preserves the 2026 launchd and runtime-path history. The standalone gateway and its demo were added in the release commit.
