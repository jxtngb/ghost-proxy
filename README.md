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

## Configuration

### Server

The default server configuration is:

```yaml
listen_address: "0.0.0.0:443"
log_level: "info"
cert_file: "configs/server.crt"
key_file: "configs/server.key"
```

### Client

```yaml
listen_address: "127.0.0.1:1080"
server_address: "127.0.0.1:443"
log_level: "info"
```

## PSK setup

The client and server must use the same hexadecimal PSK and it must decode to at least 16 bytes.

```powershell
$env:GHOST_PSK = "00112233445566778899aabbccddeeff"
```

The PSK itself is never logged by the application.

## TLS certificate

The gateway requires a certificate and private key. For local development, the E2E test uses `configs/server.crt` and `configs/server.key`.

The certificate must be trusted by the client when normal TLS verification is used. Do not commit private production keys or certificates to a public repository.

## Build and regression

```powershell
go test ./...
go build ./...
go vet ./...
powershell -ExecutionPolicy Bypass -File .\tests\e2e-actual-config.ps1
```

A successful E2E run ends with:

```
ACTUAL-CONFIG E2E TEST: PASS
```

## Nginx deployment

The repository includes a separately configured Nginx service for controlled deployment/testing:

```
deployments/nginx/
├── Dockerfile
├── nginx.conf
├── index.html
└── README.md
```

Build and run it with Docker:

```powershell
docker build -t ghost-proxy-nginx .\deployments\nginx
docker run --rm -p 8080:8080 ghost-proxy-nginx
```

Verify it:

```powershell
curl.exe http://127.0.0.1:8080/
curl.exe http://127.0.0.1:8080/health
```

The health endpoint should return:

```
ghost-nginx-ok
```

This Nginx deployment is independent of Ghost gateway authentication. The current gateway does not dynamically hand unauthenticated connections to Nginx.

For the existing Nginx setup notes, see `docs/nginx-decoy-setup.md`.

## Documentation

- `docs/socks5.md` — SOCKS5 protocol and implementation notes
- `docs/traffic-padding.md` — traffic-padding implementation notes
- `docs/nginx-decoy-setup.md` — Nginx deployment/testing notes
- `deployments/nginx/README.md` — reproducible Nginx deployment
- `tests/e2e-actual-config.ps1` — actual configuration E2E test
- `configs/client.yaml` — client configuration reference
- `configs/server.yaml` — server configuration

## Project verification status

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

The Nginx deployment is separately documented and can be verified using its health endpoint.

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

Open a pull request against `main` and record the PR, tests, owner, and verification result in the project management board.

## Security notes

- Never commit production PSKs or private TLS keys.
- Use a strong randomly generated PSK for real deployments.
- Keep TLS certificate verification enabled.
- Restrict gateway destination access according to the deployment's trust boundary.
- Use controlled destinations when demonstrating or testing forwarding.
- Review changes to cryptographic, authentication, and networking code before merging.

## License

See [LICENSE](LICENSE).
