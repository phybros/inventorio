# 📦 inventorio

Inventorio is an electronic component inventory management app designed for makers and their out-of-control component collections.

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

## Container

Build locally with:

```sh
docker build -t inventorio .
```

Run with a database URL:

```sh
docker run --rm -p 8080:8080 \
  -e DATABASE_URL='postgres://inv:inv@host.docker.internal:5432/inventory?sslmode=disable' \
  inventorio
```

## AI Usage Disclaimer

Development of this app uses AI for coding assistance.

## License

MIT

