# SOCKS5 Local Proxy

This document describes the local SOCKS5 server implemented in `pkg/socks5`. It
covers what the code actually does today, not the full RFC 1928 spec - where the
implementation is intentionally partial, that's called out explicitly.

## Purpose in Ghost Proxy

The SOCKS5 server is the **client-side entry point** of Ghost Proxy. Applications
(browsers, `curl`, SSH via `ProxyCommand`, etc.) connect to it exactly as they
would to any standard SOCKS5 proxy. It has no awareness of encryption, TLS
impersonation, or authentication - those concerns live in later pipeline stages
(uTLS transport, ChaCha20-Poly1305 framing, HMAC auth) that wrap around this
proxy's outbound connections. As of this writing, `connectToTarget` dials the
destination directly over plain TCP (`net.Dial("tcp", ...)`); the encrypted
tunnel is a separate integration point, not yet wired into this package.

Application --SOCKS5--> pkg/socks5 --plain TCP (today)--> Target
                                   --(future) uTLS tunnel--> Gateway --> Target

## Listener

- Default bind address: `127.0.0.1:1080` (set via `Server.Addr`; falls back to
  this default if empty).
- Implemented with `net.Listen("tcp", ...)`.
- Every accepted connection is handled in its own goroutine
  (`go handleConnection(conn)`), so the server supports concurrent clients.

**Known limitation:** if `listener.Accept()` itself returns an error, `Start()`
returns and the listener stops accepting new connections entirely. A single
transient accept error currently takes down the whole server rather than being
logged and skipped.

## Connection Lifecycle

Each accepted connection runs through, in order:

1. `handleGreeting` - method negotiation
2. `readRequest` - parse the CONNECT request
3. `connectToTarget` - dial the destination
4. `sendReply` - tell the client whether the dial succeeded
5. `relay` - bidirectional copy between client and target

If any step fails, the connection is closed. Only the `connectToTarget` failure
path sends a SOCKS5 reply (`replyConnectionRefused`) back to the client before
closing - greeting and request-parsing failures currently close the connection
without sending a reply. This asymmetry is worth keeping in mind for the
DPI-evasion goal: malformed/invalid probes are dropped silently rather than
routed anywhere (the Nginx decoy fallback is not yet wired in at this layer).

## Method Negotiation (Greeting)

Handled by `handleGreeting`:

1. Reads a 2-byte header: `VER | NMETHODS`.
2. Rejects anything where `VER != 0x05`.
3. Rejects a greeting that offers zero methods.
4. Reads `NMETHODS` bytes of method IDs and logs them as hex.
5. If `NO_AUTH (0x00)` is among the offered methods, replies
   `0x05 0x00` and proceeds.
6. Otherwise replies `0x05 0xFF` (no acceptable methods) and returns an error.

**Only `NO_AUTH` is supported.** Username/password (`0x02`) and other SOCKS5
auth methods are not implemented at this layer. This is intentional: Ghost
Proxy's real authentication (HMAC-SHA256 challenge-response) happens at the
transport layer, not in the SOCKS5 handshake - this proxy only needs to speak
plain, unauthenticated SOCKS5 to local applications.

## CONNECT Request

Handled by `readRequest` in `request.go`. Format: `VER | CMD | RSV | ATYP | DST.ADDR | DST.PORT`.

Validation performed, in order:
- `VER` must be `0x05`.
- `CMD` must be `0x01` (CONNECT). `BIND (0x02)` and `UDP ASSOCIATE (0x03)` are
  rejected - deliberately out of scope, since Ghost Proxy only tunnels TCP.
- `RSV` (reserved byte) must be `0x00`.
- `ATYP` must be one of the three supported address types below.

### Supported address types

| ATYP | Type   | Encoding |
|------|--------|----------|
| `0x01` | IPv4 | 4 raw bytes |
| `0x03` | Domain name | 1-byte length prefix (must be non-zero) + that many bytes, treated as text |
| `0x04` | IPv6 | 16 raw bytes |

The destination port follows as 2 bytes, big-endian.

The parsed host and port are combined into a single `host:port` string via
`net.JoinHostPort`. **DNS resolution is deferred** - it happens later, at dial
time, inside `net.Dial` (see below), not while parsing the request.

**Known limitation:** none of the read operations in this step (or the greeting
step) have a read deadline set. A client that trickles bytes very slowly can
hold a goroutine open indefinitely. Adding `conn.SetReadDeadline` is a
candidate for the Day 9 hardening pass.

## Connecting to the Target

Handled by `connectToTarget` in `connect.go`:

- Defensively re-checks that the request is non-nil and that `Command ==
  cmdConnect`, even though `readRequest` already guarantees this. Not a bug -
  just double validation, since this function could be called independently
  (e.g. from tests) without going through the full request-parsing path.
- Dials the target with `net.Dial("tcp", request.Address)`. Go's standard
  resolver handles hostname lookup transparently here for domain-name
  addresses.
- No dial timeout is set, so a target that never completes its TCP handshake
  can block the goroutine for the OS-default connect timeout (which can be
  long). `net.DialTimeout` is a candidate for later hardening, alongside the
  read-deadline issue above.

## Reply

Handled by `sendReply` in `reply.go`. Format: `VER | REP | RSV | ATYP | BND.ADDR | BND.PORT`.

Only three reply codes are currently defined:

| Value | Meaning |
|-------|---------|
| `0x00` | Succeeded |
| `0x01` | General failure |
| `0x05` | Connection refused |

The remaining RFC 1928 reply codes (network unreachable, host unreachable,
command not supported, address type not supported, etc.) are not yet defined
as constants. Since request-parsing failures currently close the connection
without any reply at all (see "Connection Lifecycle" above), these unused
codes aren't wired into a call path yet - a reasonable candidate to revisit
before Day 9 hardening if more granular client-facing errors are wanted.

The bound address in the reply is derived from the *actual* local address of
the dialed connection (`targetConn.LocalAddr()`), not from the address type
the client originally requested - this is correct RFC 1928 behavior. If a
`*net.TCPAddr` isn't available (e.g. on the connection-refused path, where an
empty `&net.TCPAddr{}` is passed in), the function falls back safely to
`0.0.0.0:0` rather than erroring.

## Bidirectional Relay

Handled by `relay` in `relay.go`:

- Two goroutines run `io.Copy` concurrently, one per direction
  (client->target, target->client).
- A buffered error channel (capacity 2) collects the result of each direction
  without blocking either goroutine.
- `relay()` returns as soon as the **first** direction finishes or errors -
  typical proxy behavior, since either side closing usually ends the session.
- The second goroutine keeps running briefly after `relay()` returns, but
  the deferred `Close()` calls on both connections (back in
  `handleConnection`) force it to exit almost immediately. Its error result is
  intentionally discarded.
- No idle or maximum-duration timeout is applied to the relay itself.

## Error Handling Summary

| Failure point | Client sees | Notes |
|---|---|---|
| Bad greeting (wrong version, no methods, unsupported auth) | `0x05 0xFF` reply where applicable, then connection closed | No decoy fallback yet |
| Bad CONNECT request (bad version/cmd/reserved byte/address type) | Connection closed, no reply sent | Asymmetric vs. connect-failure path |
| Target unreachable | `0x05 0x05` (connection refused) reply, then closed | Only failure path with a reply today |
| Successful connect | `0x05 0x00` (succeeded) reply, then relay begins | |

## Testing

Existing automated coverage (see `pkg/socks5/*_test.go`):
- `handshake_test.go` - method negotiation paths
- `request_test.go` / `request_error_test.go` - CONNECT parsing, including
  malformed requests
- `reply_test.go` - reply encoding
- `connect_test.go` - target dialing
- `relay_test.go` - bidirectional copy behavior
- `integration_test.go` - end-to-end flow

Manual/integration testing, once the proxy is running:

curl --socks5 127.0.0.1:1080 https://example.com

Planned later-stage testing (per project schedule, Day 8-9) includes probing
with `nmap` and raw/invalid TLS handshakes to confirm that invalid or
unauthenticated traffic is handled safely - at present, invalid SOCKS5 traffic
at this layer results in a closed connection rather than a decoy redirect,
since the Nginx decoy fallback is implemented at the gateway layer, not here.

## Relationship to the Broader Ghost Proxy Pipeline

This package implements only the **local, unencrypted SOCKS5 hop** between an
application and Ghost Proxy. It has no knowledge of:
- uTLS / Chrome fingerprinting (Day 4 work)
- ChaCha20-Poly1305 payload encryption or HMAC-SHA256 authentication (Day 3 work)
- Traffic padding (Day 5 work)
- The remote gateway or Nginx decoy fallback (Day 7 work)

Those layers wrap around the outbound connection this package establishes,
turning what is currently a plain `net.Dial` in `connect.go` into an
authenticated, encrypted, padded, Chrome-mimicking TLS tunnel to the Ghost
Proxy gateway. Documenting that integration point is a Day 6 (client proxy
integration) concern, not part of this file.
