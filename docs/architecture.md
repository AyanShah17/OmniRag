# OmniRAG Architecture Contract

## Ownership

```text
frontend-app/              React client and user workflows
python-rag/app/api/        HTTP adapters, validation, authentication
python-rag/app/services/   Domain orchestration and transaction boundaries
python-rag/app/parsers/    Document parsing ports and implementations
python-rag/app/chunking/   Deterministic chunking and hashing
python-rag/app/workers/    Vector indexing adapter
python-rag/app/db/         SQLAlchemy models, sessions, schema bootstrap
python-rag/app/rag/        Retrieval, reranking, prompting, generation
go-engine/internal/api/    Connector HTTP adapters
go-engine/internal/ingestion/  Python ingestion client
go-engine/internal/connectors/ Cloud storage adapters and registry
go-engine/internal/database/  Store ports and PostgreSQL/memory adapters
go-engine/internal/scheduler/ Scheduled connector orchestration
```

Python owns document parsing, chunking, versioning, relational schema, and
vector indexing. Go owns cloud connector access, scheduling, and connector
configuration. Go sends source bytes to Python; it does not duplicate document
processing rules.

## SOLID Decisions

- **Single Responsibility:** HTTP handlers validate requests and map responses;
  `DocumentIngestionService` owns ingestion orchestration; parsers, chunkers,
  vector stores, and embedding providers each have one technical concern.
- **Open/Closed:** Go connectors register a factory instead of expanding a
  central switch. New storage adapters can register without changing the
  orchestration contract.
- **Liskov Substitution:** memory and PostgreSQL stores implement the same
  narrow store ports; mock providers implement the same embedding/vector
  contracts used by production adapters.
- **Interface Segregation:** API handlers depend on connector operations and
  schedulers depend on sync operations, rather than receiving one large store
  interface for every use case.
- **Dependency Inversion:** ingestion depends on parser, chunker, and indexer
  protocols. The concrete global service is only composition-root wiring.

## ACID Guarantees

Relational document version creation is one SQLAlchemy transaction. Existing
documents are locked with `FOR UPDATE`, version numbers and content identities
are protected by unique constraints, and SQLite enables foreign keys, WAL, and
full synchronous writes for local development.

Vector stores and PostgreSQL cannot provide one distributed ACID transaction
without a distributed transaction protocol. OmniRAG therefore uses a small
recoverable saga:

1. New vectors are written before the relational commit.
2. Relational chunks are marked embedded in the same transaction.
3. A failed relational transaction compensates by deleting the vector IDs.
4. Deletion first records a durable `deleting` state, then removes vectors, then
   removes relational rows. A failed vector deletion preserves the rows and
   marks the document `error` for retry.

Sync-job completion updates the job and connector timestamp in one PostgreSQL
transaction. This gives atomicity, consistency, isolation, and durability for
each database boundary while making the unavoidable cross-store boundary
explicit.

## CIA Triad

- **Confidentiality:** production Clerk authentication, tenant/workspace
  membership checks, ACL-filtered retrieval, encrypted connector secrets,
  masked responses, restricted settings files, and no raw vector injection
  endpoint.
- **Integrity:** authenticated internal service calls, database foreign keys
  and checks, unique version/chunk identities, row locking, atomic settings
  replacement, audit records, prompt-injection handling, and defensive copies
  from the in-memory store.
- **Availability:** bounded upload and request bodies, bounded rate-limiter
  keys, database connection health checks, connector timeouts, graceful Go
  shutdown, and a development memory-store fallback. Production never silently
  falls back when required security or database configuration is invalid.

## Operational Limits

The vector/database saga is intentionally not described as distributed ACID.
Production deployments should add a formal migration tool for schema evolution
instead of relying only on `create_all` plus compatibility alterations. Real
cloud connectors and Clerk/JWKS behavior still require integration tests with
live credentials.
