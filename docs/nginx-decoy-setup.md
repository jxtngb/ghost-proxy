# Nginx Deployment Setup

## Purpose

Ghost Proxy includes a separately configured Nginx web service for controlled
deployment and testing.

The current Ghost gateway does **not** dynamically hand unauthenticated or
invalid connections to Nginx. Failed Ghost authentication remains a rejected
connection.

For the containerized project setup, Nginx listens on:

- Address: `127.0.0.1`
- Container port: `8080`
- Default Docker host port in the examples: `8080`

## Docker deployment

From the repository root:

```powershell
docker build -t ghost-proxy-nginx .\deployments\nginx
docker run --rm -p 8080:8080 ghost-proxy-nginx
```

Verify the landing page:

```powershell
curl.exe -i http://127.0.0.1:8080/
```

Verify the health endpoint:

```powershell
curl.exe -i http://127.0.0.1:8080/health
```

Expected health response:

```
ghost-nginx-ok
```

## Automated verification

Run the project-provided PowerShell health test:

```powershell
powershell -ExecutionPolicy Bypass -File .\tests\nginx-health.ps1
```

The test builds the Nginx image, starts a temporary container on host port
`18080`, checks the root page and health endpoint, verifies that the
container is running, and removes the temporary resources.

A successful run ends with:

```
NGINX HEALTH TEST: PASS
```

## Native Linux installation

For a host-based Nginx installation, install Nginx with the package manager
provided by the target Linux distribution.

The repository's container configuration is the reproducible reference setup.
Adapt the paths and service-management commands to the host distribution.

## Deployment notes

Keep Nginx independently configured from the Ghost gateway. Use a controlled
web page and controlled test environment for project demonstrations.

Do not describe this deployment as a completed Ghost authentication fallback:
that integration is not implemented or tested.
