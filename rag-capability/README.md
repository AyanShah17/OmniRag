# RAG Capability

This folder contains the backend RAG functionality consumed by a larger
application. It is not a standalone product and intentionally has no UI.

## Components

- `python-rag/` owns ingestion, parsing, chunking, storage, retrieval,
  reranking, generation, authentication, and HTTP APIs.
- `go-engine/` owns connector access, credential protection, scheduling, and
  delivery of source content to the Python service.
- `docs/architecture.md` records ownership and integration contracts.
- `.env.example` lists the environment configuration needed by the services.

## Local Verification

Start the Python API from `rag-capability/python-rag`:

```powershell
uvicorn app.main:app --host 127.0.0.1 --port 8000 --reload
```

Start the Go connector service from `rag-capability/go-engine` when connector
sync functionality is required:

```powershell
go run ./cmd/server
```

The larger application should call the Python API and internal Go ingestion
endpoint rather than depending on a bundled client.
