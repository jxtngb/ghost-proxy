# TLS fingerprint verification

The client currently builds a uTLS `HelloChrome_Auto` profile and removes
`renegotiation_info` to preserve TLS exporter access. The server advertises
only `http/1.1`, because this gateway does not implement HTTP/2.

No Chrome or Wireshark capture is available in the source-only environment
used for this change. Therefore this project does not claim that the emitted
ClientHello matches stock Chrome, and there are no JA3/JA4 values to report.
To establish evidence, capture the same TLS version and destination for a
current stock Chrome and Ghost client, record extension order and ALPN, and
compare JA3/JA4 with a pinned analyzer version. Commit the raw capture hashes,
tool versions, and resulting values here. Do not infer a match from the
`HelloChrome_Auto` profile name.
