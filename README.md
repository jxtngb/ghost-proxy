# Ghost Proxy

Ghost Proxy is a Go-based SOCKS5 proxy system that establishes an authenticated encrypted connection between a local client and a gateway. The gateway validates the session and forwards the requested TCP connection to a controlled destination.

## Current architecture

```
Application
    |
    v
Local SOCKS5 client (127.0.0.1:1080)
    |
    | SOCKS5 CONNECT
    v
Ghost client
    |
    | TLS 1.3 + authenticated Ghost protocol
    v
Ghost gateway/server (default: 0.0.0.0:443)
    |
    | authenticated destination forwarding
    v
Controlled TCP destination
```

The repository contains separate packages for configuration, logging, cryptography, framing, padding, transport, SOCKS5 handling, and gateway authentication/tunnelling.

## Implemented components

- SOCKS5 client/server handling
- TLS 1.3 transport using uTLS
- TLS exporter based session material
- PSK-based authentication
- HKDF key derivation
- HMAC authentication
- ChaCha20-Poly1305 authenticated encryption
- Framed encrypted data transport
- Traffic padding
- Gateway destination forwarding
- Authentication-success acknowledgement
- Actual client/server configuration E2E test
- Incorrect-PSK rejection test
- Go unit/integration tests and CI

Gateway destination forwarding is implemented and is part of the tested application path.

## Requirements

- Go toolchain compatible with the version declared by `go.mod`
- OpenSSL or another certificate-generation tool for local development
- A TLS certificate and private key for the gateway
- A shared hexadecimal PSK in the `GHOST_PSK` environment variable

Check the installed Go version:

```powershell
go version
```

## Clone and install dependencies

```powershell
git clone https://github.com/jxtngb/ghost-proxy.git
cd ghost-proxy
go mod download
```

## Configuration

### Server

The default server configuration is:

```yaml
listen_address: "0.0.0.0:443"
log_level: "info"
cert_file: "configs/server.crt"
key_file: "configs/server.key"
```

You can provide another configuration file with:

```powershell
go run ./cmd/server -config path/to/server.yaml
```

The `-listen` option overrides the configured listen address:

```powershell
go run ./cmd/server -config configs/server.yaml -listen 127.0.0.1:443
```

### Client

The client configuration file documents the local SOCKS5 and gateway addresses:

```yaml
listen_address: "127.0.0.1:1080"
server_address: "127.0.0.1:443"
log_level: "info"
```

The current client command uses the application defaults shown in the source, so keep the documented addresses aligned with the local test setup.

## PSK setup

The client and server must use the same PSK.

The value is supplied as hexadecimal and must decode to at least 16 bytes.

Example:

```powershell
$env:GHOST_PSK = "00112233445566778899aabbccddeeff"
```

The PSK itself is never logged by the application.

For a negative authentication test, deliberately use a different hexadecimal value on one side.

## TLS certificate

The gateway requires a certificate and private key.

For local development, the repository E2E test uses:

```
configs/server.crt
configs/server.key
```

The certificate must be trusted by the client when normal TLS verification is used. The project does not require disabling certificate verification for the E2E flow.

Do not commit private production keys or certificates to a public repository.

For production deployment, use a certificate issued by the appropriate trusted certificate authority and protect the private key with normal operating-system permissions.

## Build

Build all packages and commands:

```powershell
go build ./...
```

Run static checks:

```powershell
go vet ./...
```

Run the complete Go test suite:

```powershell
go test ./...
```

## Run the gateway

From the repository root:

```powershell
$env:GHOST_PSK = "00112233445566778899aabbccddeeff"
go run ./cmd/server
```

The server reads `configs/server.yaml` by default and listens on the configured address.

## Run the client

In a second terminal:

```powershell
$env:GHOST_PSK = "00112233445566778899aabbccddeeff"
go run ./cmd/client
```

The local SOCKS5 listener is expected at:

```
127.0.0.1:1080
```

Applications can use that address as their SOCKS5 proxy.

## Controlled end-to-end test

The repository includes an actual configuration E2E test:

```
tests/e2e-actual-config.ps1
```

It builds the actual server and client binaries, starts a controlled local destination, starts the actual gateway and client, and verifies the complete application path.

Run it from PowerShell:

```powershell
powershell -ExecutionPolicy Bypass -File .\tests\e2e-actual-config.ps1
```

The test verifies:

1. The configured gateway starts successfully.
2. The configured client starts successfully.
3. A SOCKS5 CONNECT request is accepted with the matching PSK.
4. The request passes through TLS, authentication, gateway forwarding, and the controlled destination.
5. The controlled destination response reaches the SOCKS5 client.
6. A client using an incorrect PSK is rejected.

A successful run ends with:

```
ACTUAL-CONFIG E2E TEST: PASS
```

## Full regression check

Before opening or merging a change, run:

```powershell
cd C:\Users\jesti\ghost-proxy

go test ./...
go build ./...
go vet ./...

powershell -ExecutionPolicy Bypass -File .\tests\e2e-actual-config.ps1
```

The race-enabled test suite may require a working CGO/C compiler toolchain on Windows. A local race-test compiler failure should be distinguished from a normal `go test ./...` failure.

## Nginx deployment documentation

The repository contains:

```
docs/nginx-decoy-setup.md
```

This document describes the separate Nginx web-server setup used for the project's controlled deployment/testing environment.

The current gateway implementation does not dynamically hand unauthenticated connections to Nginx. Nginx should therefore be treated as a separately configured web service until an explicitly tested integration is added.

## Documentation

- `docs/socks5.md` — SOCKS5 protocol and implementation notes
- `docs/traffic-padding.md` — traffic-padding implementation notes
- `docs/nginx-decoy-setup.md` — Nginx deployment/testing notes
- `tests/e2e-actual-config.ps1` — actual configuration E2E test
- `configs/client.yaml` — client configuration reference
- `configs/server.yaml` — server configuration

## Project verification status

The current main branch includes the completed actual-config E2E integration.

Recent verified work includes:

- PSK encoding consistency between client and server
- Gateway destination forwarding
- Authentication success acknowledgement
- Actual configured client/server E2E flow
- Matching-PSK success
- Incorrect-PSK rejection
- `go test ./...`
- `go build ./...`
- `go vet ./...`

## Development workflow

Use feature branches for changes:

```powershell
git checkout main
git pull origin main

git checkout -b <type>/<short-description>
```

After implementation:

```powershell
go test ./...
go build ./...
go vet ./...

git status --short --branch
git add .
git commit -m "type: describe the change"
git push -u origin <type>/<short-description>
```

Open a pull request against `main). Record the PR, tests, owner, and verification result in the project management board.

## Security notes

- Never commit production PSKs or private TLS keys.
- Use a strong randomly generated PSK for real deployments.
- Keep TLS certificate verification enabled.
- Restrict gateway destination access according to the deployment's trust boundary.
- Use controlled destinations when demonstrating or testing forwarding.
- Review changes to cryptographic, authentication, and networking code before merging.

## License

See [LICENSE](LICENSE).
