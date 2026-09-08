# Nginx Decoy Setup

## Purpose

Ghost Proxy uses an Nginx decoy web server as the fallback for
unauthenticated or invalid gateway connections.

The current local development setup uses:

- Address: `127.0.0.1`
- Port: `8080`
- Web root: `/var/www/ghost-decoy`

## Environment

Tested on Fedora Linux with Nginx.

## Installation

Install Nginx:

```bash
sudo dnf install nginx -y
