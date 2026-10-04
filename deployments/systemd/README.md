# systemd and Let's Encrypt

Install `ghost-server` at `/opt/ghost-proxy/ghost-server`, create a dedicated
`ghost-proxy` user, and place the server config at
`/etc/ghost-proxy/server.yaml`. Set `cert_file` and `key_file` to the active
Let's Encrypt paths, for example `/etc/letsencrypt/live/proxy.example/fullchain.pem`
and `privkey.pem`. Keep the PSK in `/etc/ghost-proxy/environment` as
`GHOST_PSK=<64 hex characters>` and make that file readable only by root and
the service group.

For a new host, obtain the certificate with Certbot's standalone HTTP-01
flow after pointing the domain's DNS at the server:

```sh
sudo certbot certonly --standalone -d proxy.example.com
sudo systemctl restart ghost-proxy
sudo certbot renew --deploy-hook "systemctl restart ghost-proxy"
```

Install the unit, enable it, and restart the service after certificate
renewal. Certificates load at startup, so a reload does not replace the
in-memory certificate.

```sh
sudo install -m 0644 deployments/systemd/ghost-proxy.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now ghost-proxy
```
