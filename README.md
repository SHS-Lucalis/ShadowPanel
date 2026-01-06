[![GameAP Logo](web/frontend/public/gameap_full.svg)](https://gameap.com)

# GameAP

[![Coverage Status](https://coveralls.io/repos/github/gameap/gameap/badge.svg?branch=main)](https://coveralls.io/github/gameap/gameap?branch=main)
![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/gameap/gameap/.github%2Fworkflows%2Ftest.yaml)
![Discord](https://img.shields.io/discord/527221172144701440)

GameAP is a free and open-source game server management panel that allows you to easily manage and deploy game servers.
It provides a user-friendly web interface for managing game servers, users, and configurations.

Demo: https://demo.gameap.com

## Requirements

You don't need to pre-install any dependencies. 
GameAP is distributed as a single binary file that includes all necessary dependencies.

You don't need any special hardware to run the application. 
A basic server with at least 1GB of RAM and a modern CPU should be sufficient for small to medium-sized deployments.

You can run the panel on different operating systems and database backends.

### Operating System

GameAP can be installed on the following operating systems:
- Linux (Ubuntu, Debian, CentOS, etc.)
- Windows Server (2016, 2019, 2022, 2025), Windows 10, Windows 11
- MacOS

### Database

GameAP supports the following databases:
- PostgreSQL
- MySQL / MariaDB
- SQLite
- Inmemory (for testing purposes only). Not persistent, data will be lost on restart.

## Quick Start with Docker

The fastest way to get started with GameAP is using Docker:

```bash
# Using Docker Compose (recommended)
docker-compose up -d

# Or pull and run the pre-built image
docker pull gameap/gameap:latest
docker run -d -p 8025:8025 \
  -e DATABASE_DRIVER=sqlite \
  -e DATABASE_URL=file:/db.sqlite?_busy_timeout=5000&_journal_mode=WAL&cache=shared \
  -e ENCRYPTION_KEY=your-secret-key \
  -e AUTH_SECRET=your-auth-secret \
  gameap/gameap:latest
```

Access GameAP at http://localhost:8025

For detailed Docker deployment instructions, see [DOCKER.md](DOCKER.md).

## Configuration

GameAP is configured via environment variables. Below are the available configuration options:

### Server Configuration

- `HTTP_HOST` - HTTP server host (default: `0.0.0.0`)
- `HTTP_PORT` - HTTP server port (default: `8025`)
- `HTTPS_PORT` - HTTPS server port (default: `443`)

### TLS Configuration

- `TLS_CERT_FILE` - Path to TLS certificate file
- `TLS_KEY_FILE` - Path to TLS private key file
- `TLS_CERT` - TLS certificate content (PEM or base64 encoded)
- `TLS_KEY` - TLS private key content (PEM or base64 encoded)
- `TLS_FORCE_HTTPS` - Force redirect HTTP to HTTPS (default: `false`)

### Database Configuration

- `DATABASE_DRIVER` - Database driver (required, options: `mysql`, `postgres`, `sqlite`, `inmemory`)
- `DATABASE_URL` - Database connection URL (required)
  - MySQL: `username:password@tcp(host:port)/database?parseTime=true`
  - PostgreSQL: `postgres://username:password@host:port/database?sslmode=disable`
  - SQLite: `file:path/to/database.db?_busy_timeout=5000&_journal_mode=WAL&cache=shared` (parameters recommended for production)
  - Inmemory: For `inmemory`, this can be left empty.

### Security Configuration

- `ENCRYPTION_KEY` - Encryption key for sensitive data
- `AUTH_SECRET` - Secret key for PASETO/JWT token generation (if not set, uses `ENCRYPTION_KEY`)
- `AUTH_SERVICE` - Authentication service type (default: `paseto`)

### RBAC Configuration

- `RBAC_CACHE_TTL` - Role-based access control cache TTL (default: `30s`)

### Cache Configuration

- `CACHE_DRIVER` - Cache driver (options: `memory`, `redis`, `postgres`, default: `memory`)

#### Redis Cache

Used when `CACHE_DRIVER` is set to `redis`.

- `CACHE_REDIS_ADDR` - Redis server address (default: `localhost:6379`)
- `CACHE_REDIS_PASSWORD` - Redis password
- `CACHE_REDIS_DB` - Redis database number (default: `0`)

#### Cache TTL

- `CACHE_TTL_RBAC` - Cache TTL for RBAC data (default: `24h`)
- `CACHE_TTL_GAMES` - Cache TTL for games (default: `48h`)
- `CACHE_TTL_NODES` - Cache TTL for nodes (default: `24h`)
- `CACHE_TTL_USERS` - Cache TTL for users (default: `6h`)
- `CACHE_TTL_PERSONAL_TOKENS` - Cache TTL for personal tokens (default: `24h`)
- `CACHE_TTL_SERVER_SETTINGS` - Cache TTL for server settings (default: `12h`)

### File Storage Configuration

- `FILES_DRIVER` - File storage driver (options: `local`, `s3`)

#### Local Storage

Used when `FILES_DRIVER` is set to `local`.

- `FILES_LOCAL_BASE_PATH` - Base path for local file storage

#### S3 Storage

Used when `FILES_DRIVER` is set to `s3`.

- `FILES_S3_ENDPOINT` - S3-compatible endpoint URL
- `FILES_S3_USE_SSL` - Use SSL for S3 connections (default: `true`)
- `FILES_S3_ACCESS_KEY_ID` - S3 access key ID
- `FILES_S3_SECRET_ACCESS_KEY` - S3 secret access key
- `FILES_S3_BUCKET` - S3 bucket name

### Legacy Configuration

- `LEGACY_PATH` - Path to legacy GameAP installation (default: `/var/www/gameap/`)
- `LEGACY_ENV_PATH` - Path to legacy .env file (default: `/var/www/gameap/.env`)

### Global API Configuration

- `GLOBAL_API_URL` - Global GameAP API URL for game updates (default: `https://api.gameap.com`)

### Logger Configuration

- `LOGGER_LEVEL` - Log level (options: `debug`, `info`, `warn`, `error`, default: `info`)
- `LOGGER_LOG_DB_QUERIES` - Enable database query logging (default: `false`)

### UI Configuration

- `DEFAULT_LANGUAGE` - Default UI language code

### Example Configuration

```bash
# Server
HTTP_HOST=0.0.0.0
HTTP_PORT=8025

# TLS (optional)
# TLS_CERT_FILE=/path/to/cert.pem
# TLS_KEY_FILE=/path/to/key.pem
# TLS_FORCE_HTTPS=true

# Database
DATABASE_DRIVER=mysql
DATABASE_URL=gameap:password@tcp(localhost:3306)/gameap?parseTime=true

# Security
ENCRYPTION_KEY=your-secure-encryption-key-here
AUTH_SECRET=your-secure-auth-secret-here
AUTH_SERVICE=paseto

# Cache
CACHE_DRIVER=memory
# For Redis cache:
# CACHE_DRIVER=redis
# CACHE_REDIS_ADDR=localhost:6379

# File Storage
FILES_DRIVER=local
FILES_LOCAL_BASE_PATH=/var/lib/gameap/files

# Legacy
LEGACY_PATH=/var/www/gameap/

# Global API
GLOBAL_API_URL=https://api.gameap.com

# Logger
LOGGER_LEVEL=info
```