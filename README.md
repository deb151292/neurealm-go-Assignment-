# Multi-Tenant User Management System (Go + PostgreSQL)

A high-performance, production-grade Multi-Tenant User Management System built in **Go (Golang)** and **PostgreSQL** using `pgxpool`. Designed with Clean Architecture, strict tenant isolation, fine-grained Role-Based Access Control (RBAC), JWT authentication with tenant-scoped access & refresh tokens, in-memory permission caching, and immutable audit logging.

---

## 📋 Features & Assignment Requirement Mapping

| Module | Requirement | Implementation Details |
| :--- | :--- | :--- |
| **Tenants** | Create and manage organizations (name, unique code, active/suspended status) | Handled in `internal/tenant`. Unique slug code validation, active/suspended lifecycle. |
| **Users** | Create, update, soft-delete, list; one user can belong to multiple tenants | Implemented in `internal/user`. Global `users` table linked via `tenant_users` with soft deletion (`deleted_at`). |
| **Roles** | Tenant-specific roles, assign multiple roles to a user per tenant | Managed in `internal/role`. `roles` are strictly scoped by `tenant_id`. User can hold multiple roles in a single tenant. |
| **Permissions** | `resource:action` permissions mapped to roles (RBAC) | Configured in `internal/permission` and `internal/role`. Standard format (e.g. `user:create`, `role:assign`). |
| **Authentication** | Email/password login, bcrypt, JWT access + refresh tokens scoped to one tenant | Built in `internal/auth`. Bcrypt password hashing, short-lived JWT access tokens + rotating refresh tokens scoped to tenant. Supports multi-tenant user login and tenant switching (`/auth/switch-tenant`). |
| **Authorization** | Middleware validating JWT and required permissions (401 vs 403) | Enforced in `internal/middleware`. Differentiates **401 Unauthorized** (missing/expired/invalid token) vs **403 Forbidden** (insufficient permissions). |
| **Tenant Isolation** | Shared DB and schema with `tenant_id` - taken from token only. Cross-tenant access returns 404 | Implemented in `internal/middleware` and repositories. Context strictly derives `tenant_id` from token claims. Cross-tenant access queries yield zero rows and return **404 Not Found** to prevent entity enumeration. |
| **Audit Logs** | Track login, user, role, assignment, and permission changes | Implemented in `internal/audit`. Records `actor`, `action`, `target`, `timestamp`, `metadata`, strictly tenant-isolated. |
| **High Volume** | Connection pooling, indexed queries, cached permission checks | Utilizes `github.com/jackc/pgx/v5/pgxpool` with tuned pool limits, composite B-tree indexes `(tenant_id, ...)`, and thread-safe in-memory permission cache (`internal/cache`). |

---

## 🏛️ Clean Architecture

The codebase follows the Clean Architecture pattern (`Handler -> Service -> Repository`):

```
.
├── cmd/
│   ├── api/main.go               # REST API entrypoint & graceful shutdown
│   └── seed/main.go              # Database seeder (tenants, roles, users)
├── migrations/
│   ├── 000001_init_schema.up.sql   # PostgreSQL normalized schema & composite indexes
│   ├── 000001_init_schema.down.sql # Rollback migration
│   └── seed.sql                  # Deterministic demo seed data
├── internal/
│   ├── config/config.go          # Environment configuration loading (.env)
│   ├── domain/                   # Entities, domain errors, and context keys
│   ├── database/                 # pgxpool connection pool and migration runner
│   ├── cache/                    # Thread-safe in-memory permission cache with TTL
│   ├── middleware/               # Auth, TenantIsolation, RBAC, RequestLogger, Recoverer
│   ├── response/                 # Standard JSON response envelope & error mapper
│   ├── validator/                # Request input validation
│   ├── auth/                     # JWT tokens, session refresh, tenant switching
│   ├── tenant/                   # Organization & tenant lifecycle
│   ├── user/                     # User management & multi-tenant membership
│   ├── role/                     # Tenant-specific roles and permission assignments
│   ├── permission/               # System permission catalog
│   ├── audit/                    # Tenant-scoped security audit trail
│   └── server/                   # Gin engine setup and route bindings
├── test/
│   ├── mock_store.go             # Mock repository store for testing
│   └── cross_tenant_test.go      # End-to-end integration & cross-tenant 404 tests
├── docs/
│   ├── DESIGN.md                 # 1-2 page Architecture & Design Document
│   ├── ERD.md                    # Database ER diagram & Schema Dictionary
│   ├── openapi.yaml              # Swagger / OpenAPI 3.0 specification
│   └── postman_collection.json   # Ready-to-import Postman Collection
├── Dockerfile                    # Multi-stage production container build
├── docker-compose.yml            # Local PostgreSQL 16 + API service
├── example.env                   # Template environment variables
├── .env                          # Local test & development environment variables
└── README.md
```

---

## 🚀 Quick Start Guide

### Prerequisites
- [Go 1.22+](https://golang.org/dl/)
- [PostgreSQL 14+](https://www.postgresql.org/) (or Docker)

### 1. Local Setup via Docker Compose (Recommended)

Run PostgreSQL and the API server in a single command:
```bash
docker compose up -d --build
```
This automatically boots PostgreSQL, applies schema migrations, loads seed data, and starts the API server on `http://localhost:8080`.

Verify health:
```bash
curl http://localhost:8080/health
```

---

### 2. Manual Local Setup

1. **Configure Environment Variables**:
   Copy `example.env` to `.env`:
   ```bash
   cp example.env .env
   ```

2. **Run Migrations & Seed Data**:
   Ensure PostgreSQL is running and `DATABASE_URL` is configured in `.env`. Then execute:
   ```bash
   go run cmd/seed/main.go
   ```

3. **Start the API Server**:
   ```bash
   go run cmd/api/main.go
   ```

---

## 📖 Interactive Swagger UI Documentation

Once the server is running, access the interactive Swagger UI directly in your browser:

- **Swagger UI**: [http://localhost:8080/swagger](http://localhost:8080/swagger) (or `/docs`)
- **OpenAPI 3.0 Spec (YAML)**: [http://localhost:8080/swagger/openapi.yaml](http://localhost:8080/swagger/openapi.yaml)
- **OpenAPI 3.0 Spec (JSON)**: [http://localhost:8080/swagger/doc.json](http://localhost:8080/swagger/doc.json)

> **Tip**: Click the **Authorize** button in Swagger UI to paste a JWT token obtained from `/api/v1/auth/login`. Authorization persists across requests so you can test all protected endpoints directly in the browser!

---

## 🧪 Running Tests

The test suite includes comprehensive unit tests and end-to-end integration tests that explicitly verify **cross-tenant isolation** and **authorization edge cases**:

```bash
# Run all unit and integration tests
go test -v ./...
```

### Key Test Scenarios Covered
- **Cross-Tenant Access Test**: Verifies that when Tenant B attempts to read (`GET`), update (`PUT`), or delete (`DELETE`) Tenant A's users or roles, the API returns **`404 Not Found`** (zero data leakage).
- **Authorization 401 vs 403 Test**: Verifies that missing or invalid tokens return `401 Unauthorized`, while authenticated users lacking required permissions return `403 Forbidden`.
- **Audit Log Isolation**: Verifies that Tenant B never sees any audit log entries belonging to Tenant A.
- **Multi-Tenant User Switching**: Verifies that a user belonging to multiple organizations can log in to one tenant and switch to another tenant (`/api/v1/auth/switch-tenant`), receiving new scoped tokens.
- **Soft Deletion Isolation**: Verifies that soft-deleted users cannot be accessed or listed.

---

## 🔑 Demo Seed Credentials

All seeded accounts have the default password: **`Password123!`**

| User Email | Tenant Code | Tenant Name | Assigned Role | Capabilities |
| :--- | :--- | :--- | :--- | :--- |
| `alice@multitenant.com` | `acme-corp` | Acme Corporation | **Admin** | Full Admin rights in Acme |
| `alice@multitenant.com` | `globex` | Globex Corporation | **Auditor** | Read-only audit access in Globex |
| `bob@acme.com` | `acme-corp` | Acme Corporation | **Admin** | Full Admin rights in Acme |
| `charlie@globex.com` | `globex` | Globex Corporation | **Auditor** | Read-only in Globex |

---

## 📡 API Reference Overview

Base URL: `http://localhost:8080/api/v1`

### Authentication (`/auth`)
- `POST /auth/login`: Authenticate with email, password, and optional `tenant_code`.
- `POST /auth/refresh`: Rotate refresh token and get a fresh access token.
- `POST /auth/switch-tenant`: Switch active tenant context for multi-tenant users.
- `GET /auth/me`: Inspect active user profile, tenant scope, roles, and permissions.
- `POST /auth/logout`: Revoke session refresh token and purge permission cache.

### Users (`/users`)
- `POST /users`: Create user in current tenant (requires `user:create`).
- `GET /users`: List users in tenant with pagination & search (requires `user:read`).
- `GET /users/{id}`: Get user details (cross-tenant access returns `404`).
- `PUT /users/{id}`: Update user (cross-tenant access returns `404`).
- `DELETE /users/{id}`: Soft delete user (cross-tenant access returns `404`).
- `POST /users/{id}/roles`: Assign multiple roles to user (requires `role:assign`).

### Roles & Permissions (`/roles`, `/permissions`)
- `POST /roles`: Create tenant-specific role (requires `role:create`).
- `GET /roles`: List roles in tenant (requires `role:read`).
- `GET /roles/{id}`: Get role details (cross-tenant access returns `404`).
- `PUT /roles/{id}`: Update role (cross-tenant access returns `404`).
- `DELETE /roles/{id}`: Delete role (system roles protected).
- `POST /roles/{id}/permissions`: Map `resource:action` permissions to role.
- `GET /permissions`: Catalog of available system permissions.

### Audit Logs (`/audit-logs`)
- `GET /audit-logs`: Query tenant-scoped audit trail (actor, action, target, metadata, timestamp).

---

## 📚 Deliverables & Documentation Links

- **Design Document (1–2 pages)**: [docs/DESIGN.md](file:///e:/codes/Assignments/priyanka/neurealm-go-Assignment-/docs/DESIGN.md)
- **Database ER Diagram & Schema**: [docs/ERD.md](file:///e:/codes/Assignments/priyanka/neurealm-go-Assignment-/docs/ERD.md)
- **Swagger / OpenAPI 3.0 Specification**: [docs/openapi.yaml](file:///e:/codes/Assignments/priyanka/neurealm-go-Assignment-/docs/openapi.yaml)
- **Postman Collection**: [docs/postman_collection.json](file:///e:/codes/Assignments/priyanka/neurealm-go-Assignment-/docs/postman_collection.json)
- **Database Migrations**: [migrations/000001_init_schema.up.sql](file:///e:/codes/Assignments/priyanka/neurealm-go-Assignment-/migrations/000001_init_schema.up.sql)
- **Seed Data Script**: [migrations/seed.sql](file:///e:/codes/Assignments/priyanka/neurealm-go-Assignment-/migrations/seed.sql)
