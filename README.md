# Inventorio

Inventorio is a self-hosted inventory app for electronic components and maker
parts collections. It runs as a single Go web app backed by PostgreSQL.

## Features

- Component inventory with categories, storage locations, manufacturer part
  numbers, datasheets, notes, quantities, and low-stock tracking.
- Category-specific attributes with numeric, text, boolean, and enum fields,
  including inherited attributes from parent categories.
- Search and filtering across components and category-specific attributes.
- DigiKey cart CSV and Mouser order XLS importers with preview, editable fields,
  category suggestions, attribute entry, and quantity merging for matching MPNs.
- Duplicate component detection and interactive merge previews for consolidating
  records that share the same MPN.
- Project and BOM tracking with buildability checks, shortage reporting, project
  duplication, and build execution that subtracts used parts from inventory.
- Hierarchical storage locations with printable location labels.
- Audit history for inventory, import, merge, project, and build actions.
- Optional authentication through GitHub or Google OAuth, or a trusted reverse
  proxy.

## Quick Start

Start PostgreSQL with the included Compose file:

```sh
docker compose up -d
```

Then run Inventorio locally:

```sh
go build
DATABASE_URL='postgres://inv:inv@localhost:5432/inventory?sslmode=disable' ./inventorio
```

Open `http://localhost:8080`.

To run the published container instead:

```sh
docker run --rm -p 8080:8080 \
  -e DATABASE_URL='postgres://inv:inv@host.docker.internal:5432/inventory?sslmode=disable' \
  ghcr.io/phybros/inventorio:latest
```

## Docker Compose

Use Inventorio with PostgreSQL in one Compose stack:

Create `docker-compose.yaml`

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
    restart: on-failure
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://inv:inv@db:5432/inventory?sslmode=disable
    depends_on:
      - db
```

## Configuration

Inventorio listens on `:8080` by default.

| Variable | Default | Description |
| --- | --- | --- |
| `LISTEN_ADDR` | `:8080` | HTTP listen address. |
| `DATABASE_URL` | unset | Full PostgreSQL connection URL. Takes precedence over individual DB variables. |
| `DB_HOST` | `localhost` | PostgreSQL host when `DATABASE_URL` is unset. |
| `DB_PORT` | `5432` | PostgreSQL port. |
| `DB_NAME` | `inventory` | Database name. |
| `DB_USER` | `inv` | Database user. |
| `DB_PASSWORD` | `inv` | Database password. |
| `DB_SSLMODE` | `disable` | PostgreSQL SSL mode. |

Example:

```sh
DATABASE_URL='postgres://inv:inv@localhost:5432/inventory?sslmode=disable'
```

## Authentication

Inventorio defaults to no authentication so existing self-hosted installs keep
working:

```sh
INVENTORIO_AUTH_MODE=disabled
```

Do not expose a disabled-mode deployment directly to the internet. Use OAuth or
a trusted authenticating reverse proxy for internet-facing deployments.

### OAuth

OAuth mode supports GitHub and Google. Configure at least one provider, set a
public URL, and restrict users with allowed emails or domains.

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

Provider variables are optional individually, but each enabled provider needs
both a client ID and client secret. GitHub users need a primary verified email;
Google users need a verified Google email.

Generate a session secret with:

```sh
openssl rand -base64 32
```

Provider callback URLs are derived from `INVENTORIO_PUBLIC_URL`:

```text
https://inventory.example.com/auth/github/callback
https://inventory.example.com/auth/google/callback
```

For local testing with `INVENTORIO_PUBLIC_URL=http://localhost:8080`:

```text
GitHub callback URL:
http://localhost:8080/auth/github/callback

Google JavaScript origin:
http://localhost:8080

Google redirect URI:
http://localhost:8080/auth/google/callback
```

### Reverse Proxy

Proxy mode trusts an upstream proxy to authenticate the user and send the
authenticated user's email in `X-Forwarded-User` by default.

```sh
INVENTORIO_AUTH_MODE=proxy
INVENTORIO_ALLOWED_DOMAINS=example.com
```

Set `INVENTORIO_PROXY_AUTH_HEADER` if your proxy uses a different header:

```sh
INVENTORIO_PROXY_AUTH_HEADER=cf-access-authenticated-user-email
```

Only use proxy mode when Inventorio is not directly reachable by clients.
Configure the proxy to strip any incoming client-supplied auth header before
setting the trusted value.

### Auth Variables

| Variable | Default | Description |
| --- | --- | --- |
| `INVENTORIO_AUTH_MODE` | `disabled` | `disabled`, `oauth`, or `proxy`. |
| `INVENTORIO_PUBLIC_URL` | unset | Required for OAuth. Used to build redirect URLs. |
| `INVENTORIO_SESSION_SECRET` | unset | Required for OAuth. Must be at least 32 characters. |
| `INVENTORIO_GITHUB_CLIENT_ID` | unset | GitHub OAuth client ID. |
| `INVENTORIO_GITHUB_CLIENT_SECRET` | unset | GitHub OAuth client secret. |
| `INVENTORIO_GOOGLE_CLIENT_ID` | unset | Google OAuth client ID. |
| `INVENTORIO_GOOGLE_CLIENT_SECRET` | unset | Google OAuth client secret. |
| `INVENTORIO_ALLOWED_EMAILS` | unset | Comma-separated email allowlist. |
| `INVENTORIO_ALLOWED_DOMAINS` | unset | Comma-separated domain allowlist. |
| `INVENTORIO_AUTH_ALLOW_ALL_USERS` | `false` | Allows any authenticated OAuth or proxy user. Avoid for internet-facing installs. |
| `INVENTORIO_PROXY_AUTH_HEADER` | `X-Forwarded-User` | Header trusted in proxy mode as the authenticated user's email. |
| `INVENTORIO_SESSION_COOKIE_NAME` | `inventorio_session` | Session cookie name. |
| `INVENTORIO_COOKIE_SECURE` | `auto` | `auto`, `true`, or `false`. |

Allowlist matching is case-insensitive. Password login is not included in
Inventorio 1.0.

## Building

Build the image locally:

```sh
docker build -t inventorio .
```

To include build metadata in the footer and `/healthz` response:

```sh
docker build -t inventorio \
  --build-arg VERSION="$(git describe --tags --always --dirty)" \
  --build-arg COMMIT="$(git rev-parse HEAD)" \
  --build-arg BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  .
```

In GitHub Actions, pass the same args from the workflow context:

```yaml
build-args: |
  VERSION=${{ github.ref_name }}
  COMMIT=${{ github.sha }}
  BUILD_DATE=${{ github.event.repository.updated_at }}
```

Plain `go build` defaults to version `dev` and uses Go's embedded VCS revision
when available.

## Notes

AI coding assistance has been used in the creation of this app.

## License

MIT
