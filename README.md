# ⚡ OmniRAG — Enterprise Dynamic Multi-Tenant RAG SaaS

[![Python 3.10+](https://img.shields.io/badge/Python-3.10%2B-3776AB?style=for-the-badge&logo=python&logoColor=white)](https://python.org)
[![Go 1.22+](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://golang.org)
[![React 18](https://img.shields.io/badge/React-18-61DAFB?style=for-the-badge&logo=react&logoColor=black)](https://react.dev)
[![TailwindCSS](https://img.shields.io/badge/Tailwind_CSS-3.4-38B2AC?style=for-the-badge&logo=tailwind-css&logoColor=white)](https://tailwindcss.com)
[![Pinecone](https://img.shields.io/badge/Pinecone-Serverless-000000?style=for-the-badge&logo=pinecone&logoColor=white)](https://pinecone.io)
[![FastEmbed ONNX](https://img.shields.io/badge/FastEmbed-ONNX_%240_Cost-FF6F00?style=for-the-badge)](https://qdrant.github.io/fastembed/)

**OmniRAG** is an enterprise-grade, multi-tenant Dynamic RAG SaaS platform designed for high-concurrency cloud document scanning, incremental version diffing, and grounded citation streaming. 

Built with a **hybrid Go + Python architecture**, OmniRAG connects directly to **AWS S3, Azure Blob, Supabase Storage, and Confluence Wiki**, scanning permissible directories and synchronizing documents at **near-zero cost** by diffing content at the SHA-256 chunk level.

See [docs/architecture.md](docs/architecture.md) for the ownership boundaries, SOLID decisions, ACID transaction model, and CIA controls.

See [docs/branching.md](docs/branching.md) for the development, pre-production, and production promotion workflow.

See [docs/circleci.md](docs/circleci.md) for the CircleCI and GitHub setup, branch gates, and deployment contexts.

---

## 🏛️ System Architecture Topology

```
React UI ── REST/SSE ──> Python RAG Core ──> PostgreSQL + Vector Store
    │                         ▲
    └── connector control ──> Go Connector Engine
                                  │
                                  └── fetched source bytes ──┘
```

Python is the single owner of parsing, chunking, versioning, schema initialization,
and vector indexing. Go owns connector credentials, cloud discovery, scheduling, and
transport; it sends fetched source content to Python through an authenticated internal
endpoint. This keeps one canonical chunk/hash policy across uploads and connector syncs.

---

## ✨ Key Capabilities

### 1. ⚡ Zero-Cost Incremental Chunk-Level Diffing
When a 100-page policy or document is updated by 1 sentence, legacy RAG systems re-embed the entire file. OmniRAG parses and generates deterministic **SHA-256 chunk hashes**:
- Identical chunks are reused across versions at **$0 embedding cost**.
- Only genuinely new or modified chunks are vectorized and sent to Pinecone.
- In-memory set diffing yields **60% to 95% cost reductions** on enterprise repositories.

### 2. 🌐 Multi-Cloud Knowledge Connectors (Go Engine)
- **AWS S3**: High-throughput prefix scanning with AWS Signature v4.
- **Azure Blob Storage**: Shared key authenticated container enumeration.
- **Supabase Storage**: Authenticated REST API listing and signed URL downloads.
- **Confluence Cloud**: CQL-driven space hierarchy scanning and storage-format parsing.
- **Local Storage / Direct Upload**: Fast drag-and-drop file ingestion.

### 3. 🎯 Precision Grounded Citations & Sub-Second Re-ranking
- **FlashRank Neural Cross-Encoder** (`ms-marco-TinyBERT-L-2-v2`): Re-ranks candidate vectors in $<20\text{ms}$ locally with 0 external API dependencies.
- **Grounded Source Cards**: Streams Server-Sent Events (SSE) tokens directly with interactive citation cards showing exact document title, section heading, page number, and text snippet.

### 4. 🎛️ Live AI & Key Configuration Control
- Configure **FastEmbed ONNX** ($0 local cost), **OpenRouter**, **Groq**, and **OpenAI**.
- Update API keys and provider settings via the **CLI Wizard** (`omnirag-config`) or the **Web Settings Modal**; restart the Python service to apply changes.

### 5. 📦 Native Windows (MSI) & Linux (RPM) Packaging
- **Windows**: WiX installer definition (`.msi`) and portable bundle with automated background launcher.
- **Linux**: RPM specification file (`.rpm`) and **Systemd** daemon service unit (`omnirag.service`).

---

## 🚀 Quickstart Guide

### Prerequisites
- **Python 3.10+** (with `uv` or `pip`)
- **Go 1.21+**
- **Node.js 18+** & `npm`

---

### Step 1: Clone the Repository
```bash
git clone https://github.com/AyanShah17/OmniRag.git
cd OmniRag
```

### Step 2: Configure Environment
Copy the template configuration file:
```bash
# On Linux/macOS
cp .env.example .env

# On Windows
copy .env.example .env
```

You can also run the interactive CLI configuration wizard:
```bash
python packaging/config_manager/omnirag_config.py
```

---

### Step 3: Run Interactive CLI Demonstration
Experience document version ingestion, chunk diffing math, and live streaming citations in terminal:
```bash
# Create Python virtual environment
cd python-rag
uv venv
.\.venv\Scripts\activate   # Or source .venv/bin/activate on Linux
uv pip install -r requirements.txt

# Run live demo
python ../scripts/demo_e2e_rag.py
```

---

### Step 4: Run Complete Stack (Backend & Web UI)

#### Terminal 1 — Python RAG Core & Auto-Served Web UI (Port 8000)
```bash
cd python-rag
.\.venv\Scripts\python.exe -m uvicorn app.main:app --host 0.0.0.0 --port 8000 --reload
```

#### Terminal 2 — Go Connector Engine (Port 8080)
```bash
cd go-engine
go run cmd/server/main.go
```

#### Terminal 3 (Optional) — Frontend Vite Dev Server (Port 3000)
```bash
cd frontend-app
npm install
npm run dev
```

Open **`http://localhost:8000`** in your browser to access the ChatGPT-style interface!

---

## 📦 Native OS Installers

### Windows MSI Package Build
```powershell
powershell.exe -ExecutionPolicy Bypass -File .\packaging\windows\build_msi.ps1
```
Output: `packaging\windows\output\OmniRAG-Windows-x64.zip` / `OmniRAG-1.0.0-x64.msi`

### Linux RPM Package Build
```bash
bash packaging/linux/build_rpm.sh
```
Output: `packaging/linux/output/rpmbuild/RPMS/x86_64/omnirag-1.0.0-1.x86_64.rpm`

---

## 🧪 Automated Test Verification

Run complete test suites across both Go and Python engines:

### Python RAG & API Integration Tests
```bash
.\python-rag\.venv\Scripts\python.exe -m pytest -v -o asyncio_mode=auto .\python-rag\tests\
============================== 22 passed ==============================
```

### Go Connector, Authorization, and Security Tests
```bash
cd go-engine
go test -v ./...
PASS
```

---

## 📡 REST API & SSE Endpoints

| Method | Endpoint | Service | Description |
|---|---|---|---|
| `GET` | `/api/v1/healthz` | Python / Go | Health status & active AI provider info |
| `POST` | `/api/v1/documents/upload` | Python Core | Multipart document upload with chunk-level diffing |
| `GET` | `/api/v1/documents` | Python Core | List all indexed documents in workspace |
| `GET` | `/api/v1/documents/{id}/versions` | Python Core | Fetch historical document versions |
| `POST` | `/api/v1/chat/conversations` | Python Core | Create a conversation session |
| `GET` | `/api/v1/chat/conversations` | Python Core | List the current user's conversations |
| `POST` | `/api/v1/chat/completions/stream` | Python Core | Real-time SSE token stream with citations |
| `GET` | `/api/v1/settings` | Python Core | Get active system providers and masked keys |
| `POST` | `/api/v1/settings` | Python Core | Dynamically update AI providers and credentials |
| `POST` | `/api/v1/connectors` | Go Engine | Register S3, Azure, Supabase, Confluence connector |
| `POST` | `/api/v1/connectors/test` | Go Engine | Test cloud storage bucket connectivity |
| `POST` | `/api/v1/connectors/{id}/sync` | Go Engine | Trigger immediate online crawler sync |

*A ready-to-import Postman Collection is available in [`scripts/postman_collection.json`](scripts/postman_collection.json).*

---

## 📄 License
OmniRAG is enterprise-ready software licensed under the MIT License.
