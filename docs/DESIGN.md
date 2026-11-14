# Multi-Tenant User Management System - Design Document

## 1. Executive Summary & Architecture Overview

The **Multi-Tenant User Management System** provides high-throughput authentication, tenant management, tenant-isolated role-based access control (RBAC), and compliance-ready audit logging. The system is designed using **Clean Architecture** principles in Go, with PostgreSQL as the relational storage layer accessed via `pgxpool`.

```
                    ┌────────────────────────────────────────────────────────┐
                    │                      HTTP Clients                      │
                    └───────────────────────────┬────────────────────────────┘
                                                │ REST JSON
                                                ▼
┌────────────────────────────────────────────────────────────────────────────────────────────┐
│ Handler Layer (HTTP Presentation)                                                         │
│   ├── Router (Gin HTTP Engine)                                                             │
│   ├── Middlewares: Auth (JWT), EnforceTenantIsolation, RBAC (401 vs 403), RequestLogger   │
│   └── Handlers: AuthHandler, TenantHandler, UserHandler, RoleHandler, AuditHandler         │
└───────────────────────────────────────────────┬────────────────────────────────────────────┘
                                                │ Request DTOs / Context
                                                ▼
┌────────────────────────────────────────────────────────────────────────────────────────────┐
│ Service Layer (Business Logic & Domain Orchestration)                                      │
│   ├── AuthService (Bcrypt verification, multi-tenant resolution, token issuance)           │
│   ├── TenantService (Lifecycle, default role bootstrap)                                    │
│   ├── UserService (Soft delete, multi-tenant link)                                         │
│   ├── RoleService (RBAC resolution, tenant role hierarchy)                                 │
│   ├── PermissionCache (Fast thread-safe in-memory TTL cache with fine-grained eviction)    │
│   └── AuditService (Async non-blocking security event recording)                           │
└───────────────────────────────────────────────┬────────────────────────────────────────────┘
                                                │ Domain Entities & Query Params
                                                ▼
┌────────────────────────────────────────────────────────────────────────────────────────────┐
│ Repository Layer (Storage Abstraction)                                                     │
│   ├── Repository Interfaces (Inversion of Control for testability & database portability)  │
│   └── PostgreSQL pgxpool Implementations (Composite tenant indexed queries)                │
└───────────────────────────────────────────────┬────────────────────────────────────────────┘
                                                │ SQL / Binary Protocol
                                                ▼
                                    ┌───────────────────────┐
                                    │ PostgreSQL 16 Cluster │
                                    └───────────────────────┘
```

---

## 2. Tenant Isolation Strategy

### 2.1 Shared Database & Schema with `tenant_id`
The system utilizes a **Shared Database, Shared Schema** tenancy pattern. Every tenant-scoped entity (`roles`, `user_roles`, `tenant_users`, `refresh_tokens`, `audit_logs`) stores a `tenant_id UUID` foreign key referencing the `tenants` table.

### 2.2 Token-Authoritative Tenant Resolution
A common vulnerability in multi-tenant architectures is trusting client-supplied tenant identifiers in URL path parameters, query strings, or custom headers (`X-Tenant-ID`). In this system:
1. **The tenant context is extracted exclusively from the cryptographically signed JWT token.**
2. When the `Auth` middleware validates the bearer access token, it injects the verified `TenantID` and `TenantCode` into `context.Context`.
3. All repository queries strictly enforce `WHERE tenant_id = :tokenTenantID`.

### 2.3 Strict 404 on Cross-Tenant Access (Zero Data Leakage)
A critical requirement of secure multi-tenancy is preventing **entity enumeration** and **information disclosure**.
- If Tenant B attempts to read, modify, or delete a resource belonging to Tenant A (e.g. `GET /api/v1/users/<tenantA_user_id>`), the database query filters by `u.id = :id AND tu.tenant_id = :tokenTenantID`.
- Because the row does not match both conditions, the database returns `0 rows`.
- The handler responds with **`404 Not Found`** rather than `403 Forbidden`.
- **Justification**: Returning `403 Forbidden` confirms to an attacker that an entity ID exists in another tenant. Returning `404 Not Found` treats foreign resources as non-existent, eliminating tenant enumeration.

---

## 3. RBAC & Token Design

### 3.1 Permission Model: `resource:action`
Permissions are defined as granular string tuples formatted as `resource:action`, such as:
- `user:create`, `user:read`, `user:update`, `user:delete`
- `role:create`, `role:read`, `role:update`, `role:delete`, `role:assign`
- `tenant:read`, `tenant:update`
- `audit:read`

### 3.2 Tenant-Specific Roles & Multi-Role Aggregation
- Roles are strictly scoped to a tenant (`tenant_id`). Tenant A's "Admin" role is distinct from Tenant B's "Admin" role.
- A user can be assigned **multiple roles within a single tenant**.
- The user's effective permissions within that tenant are the **mathematical union** of permissions granted by all assigned roles:
  $$\text{Effective Permissions} = \bigcup_{r \in \text{UserRoles}} \text{Permissions}(r)$$

### 3.3 Authentication & Token Flow
1. **Login**: User submits `email` and `password`. The password hash is verified with `bcrypt`.
2. **Tenant Scope Resolution**:
   - If `tenant_code` is provided: Validates active membership in that tenant.
   - If omitted and user has multiple active tenants: Returns `requires_tenant_selection: true` with tenant options.
   - If single tenant: Automatically resolves the tenant.
3. **JWT Access Token**: Short-lived (15 minutes). Contains claims:
   - `sub`: User ID
   - `tid`: Tenant ID (authoritative)
   - `tcode`: Tenant Code
   - `roles`: Assigned role names
   - `permissions`: Active permission list
4. **Refresh Token**: Long-lived (7 days). Secure 32-byte cryptographic entropy, stored hashed (SHA-256) in `refresh_tokens`. Supports immediate revocation and **token rotation** upon refresh.
5. **Tenant Switching (`/auth/switch-tenant`)**: Allows an authenticated user to transition their session to another tenant they belong to, issuing fresh scoped tokens.

### 3.4 Authorization Middleware: 401 vs 403 Differentiation
- **401 Unauthorized**: Missing `Authorization` header, invalid bearer format, expired access token, tampered signature.
- **403 Forbidden**: Token is completely valid, but the user's role lacks the permission required for the endpoint (e.g. attempting `user:delete` with a `Viewer` role).

---

## 4. High-Volume Performance & Scaling Strategy

### 4.1 Database Connection Pooling with `pgxpool`
PostgreSQL connection creation incurs process fork and TLS handshake costs. The system utilizes `github.com/jackc/pgx/v5/pgxpool` with tuned parameters:
- `MaxConns`: 25–100 per API replica depending on connection capacity.
- `MinConns`: Pre-warmed standby connections avoiding cold-start latency.
- `MaxConnLifetime` & `MaxConnIdleTime`: Periodic turnover preventing resource leaks.
- `HealthCheckPeriod`: Continuous background pool health evaluation.

### 4.2 In-Memory Permission Caching
Evaluating permissions on every API request from the database creates high query volume. The system incorporates `cache.PermissionCache`:
- Keyed by `tenant_id:user_id` with `sync.RWMutex` for concurrent, non-blocking reads.
- **Cache Invalidation**:
  - Updating a user's role immediately invalidates that user's cache key.
  - Modifying a role's permissions invalidates all cache entries for that tenant.
  - Soft-deleting a user immediately purges cached permissions.
- Background ticker cleans expired entries automatically.

### 4.3 Database Indexing Strategy
All tables queried under tenant context have composite B-tree indexes:
- `(tenant_id, user_id)` on `tenant_users` and `user_roles`
- `(tenant_id, deleted_at)` to accelerate active user listings
- `(tenant_id, created_at DESC)` on `audit_logs` for instant paginated audit trails

---

## 5. Key Trade-offs & Next Steps

| Architectural Decision | Chosen Strategy | Trade-off / Consideration |
| :--- | :--- | :--- |
| **Tenancy Isolation** | Shared DB + Shared Schema | Lower operational complexity & cost than DB-per-tenant, but requires diligent query scoping. |
| **Cross-Tenant Handling** | Return 404 Not Found | Prevents ID enumeration; slight developer ambiguity during debugging. |
| **Permission Storage** | Token Claims + In-Memory Cache | Blazing fast verification; token permission changes apply upon refresh or cache lookup. |
| **Audit Logging** | Asynchronous Goroutine Worker | Zero latency impact on main HTTP responses; handled in-process. |

### Next Steps for Enterprise Scale
1. **Distributed Caching (Redis Cluster)**: Transition in-memory permission cache to Redis Cluster with pub/sub invalidation for multi-instance deployments.
2. **PostgreSQL Read Replicas**: Route read-only queries (listing users, roles, audit trails) to read replicas via pgx connection pooling.
3. **Partitioning**: Partition the `audit_logs` table by `tenant_id` hash or range by `created_at` (monthly partitions) for archiving.
4. **Row-Level Security (RLS)**: Enforce PostgreSQL Row-Level Security (`SET LOCAL app.current_tenant_id = ...`) as a second defense-in-depth layer.
