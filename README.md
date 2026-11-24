# starTeam Control Plane

A small Go control plane for local application components. The gateway checks a primary and standby backend, routes read requests to a ready backend, and exposes a status endpoint. This standalone example combines the original macOS LaunchAgent and runtime-path code with a synthetic two-backend gateway.

```sh
go run ./cmd/starteam-control demo
go test ./...
```

The demo starts two temporary loopback services, simulates a primary failure, and prints both routing decisions. To run the gateway against your own two services, use `go run ./cmd/starteam-control --config examples/config.json` and substitute origins that implement `/healthz`. The gateway listens on loopback. Its `/_control/status` endpoint reports readiness. A probe picks the backend for each read request.

On macOS, `write-plist` writes a LaunchAgent into a user-selected runtime directory. Build the binary first, then pass `--root`, an absolute `--binary` path and an absolute `--config` path. The command only renders the plist.

The release preserves the related 2026 launchd and path-management commits. The standalone gateway and demo are in the release commit.
