# 📦 inventorio

Inventorio is an electronic component inventory management app designed for makers and their out-of-control component collections.

## Usage

Run with Docker Compose using the included PostgreSQL service:

```yaml
services:
  db:
    image: postgres:18-alpine
    environment:
      POSTGRES_DB: inventory
      POSTGRES_USER: inv
      POSTGRES_PASSWORD: inv

  inventorio:
    image: ghcr.io/phybros/inventorio:latest
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://inv:inv@db:5432/inventory?sslmode=disable
    depends_on:
      - db
```

Or run the container directly and pass a database URL:

```sh
docker run --rm -p 8080:8080 \
  -e DATABASE_URL='postgres://inv:inv@host.docker.internal:5432/inventory?sslmode=disable' \
  ghcr.io/phybros/inventorio:latest
```

Or just build and run the binary, no Docker required:

```sh
go build
DATABASE_URL='postgres://inv:inv@host.docker.internal:5432/inventory?sslmode=disable' ./inventorio
```

Build the Docker container locally with:

```sh
docker build -t inventorio .
```

## Configuration

Inventorio listens on `:8080` by default. Set `LISTEN_ADDR` to override it.

Database configuration can be provided as a full PostgreSQL URL:

```sh
DATABASE_URL=postgres://inv:inv@localhost:5432/inventory?sslmode=disable
```

If `DATABASE_URL` is not set, Inventorio builds the connection URL from:

```sh
DB_HOST=localhost
DB_PORT=5432
DB_NAME=inventory
DB_USER=inv
DB_PASSWORD=inv
DB_SSLMODE=disable
```

## Authentication

Inventorio defaults to no authentication for compatibility with existing
self-hosted installs:

```sh
INVENTORIO_AUTH_MODE=disabled
```

**Do not expose a disabled-mode deployment directly to the internet. For
internet-facing deployments, use OAuth or a trusted authenticating reverse proxy.**

### OAuth

OAuth mode supports GitHub and Google. At least one provider must be configured,
and authenticated users must be constrained with an allowlist unless you
explicitly opt into allowing everyone.

```sh
INVENTORIO_AUTH_MODE=oauth
INVENTORIO_PUBLIC_URL=https://inventory.example.com
INVENTORIO_SESSION_SECRET=replace-with-at-least-32-random-characters

INVENTORIO_GITHUB_CLIENT_ID=...
INVENTORIO_GITHUB_CLIENT_SECRET=...

INVENTORIO_GOOGLE_CLIENT_ID=...
INVENTORIO_GOOGLE_CLIENT_SECRET=...

INVENTORIO_ALLOWED_EMAILS=alice@example.com,bob@example.com
INVENTORIO_ALLOWED_DOMAINS=example.org
```

Provider entries are optional individually, but each configured provider needs
both a client ID and client secret. GitHub logins require a primary verified
email. Google logins require a verified Google email.

`INVENTORIO_SESSION_SECRET` signs OAuth state and must be at least 32
characters. Generate a production value with:

```sh
openssl rand -base64 32
```

Set provider callback URLs from `INVENTORIO_PUBLIC_URL`:

```text
GitHub authorization callback URL:
https://inventory.example.com/auth/github/callback

Google authorized redirect URI:
https://inventory.example.com/auth/google/callback
```

For local testing with `INVENTORIO_PUBLIC_URL=http://localhost:8080`, use:

```text
GitHub authorization callback URL:
http://localhost:8080/auth/github/callback

Google authorized JavaScript origin:
http://localhost:8080

Google authorized redirect URI:
http://localhost:8080/auth/google/callback
```

Google origins include only the scheme, host, and optional port. Redirect URIs
include the full callback path and must exactly match the URL Inventorio sends
to Google. GitHub only needs the full callback URL.

### Reverse Proxy

Proxy mode trusts a reverse proxy to authenticate the user and pass:

```http
X-Forwarded-User: authenticated-user@example.com
```

Example:

```sh
INVENTORIO_AUTH_MODE=proxy
INVENTORIO_ALLOWED_DOMAINS=example.com
```

**Proxy mode is only safe when Inventorio is not directly reachable by clients.
Configure the proxy to strip any incoming client-supplied `X-Forwarded-User`
header before setting the trusted value.**

### Auth Options

```sh
INVENTORIO_SESSION_COOKIE_NAME=inventorio_session
INVENTORIO_COOKIE_SECURE=auto
```

`INVENTORIO_COOKIE_SECURE` accepts `auto`, `true`, or `false`. `auto` uses secure
cookies for HTTPS requests, `X-Forwarded-Proto: https`, or an HTTPS
`INVENTORIO_PUBLIC_URL`.

Allowlist matching is case-insensitive. `INVENTORIO_AUTH_ALLOW_ALL_USERS=true`
allows any successfully authenticated OAuth or proxy user and should be avoided
for internet-facing deployments. Password login is not included in Inventorio
1.0.

## AI Usage Disclaimer

AI coding assistance has been used in the creation of this app.

## License

MIT
