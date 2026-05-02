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

Build locally with:

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

## AI Usage Disclaimer

AI coding assistance has been used in the creation of this app.

## License

MIT
