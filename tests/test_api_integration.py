import io
import os
import sys
import uuid
import pytest
from httpx import AsyncClient, ASGITransport

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../python-rag")))

from app.main import app
from app.db.init_db import init_database


@pytest.mark.asyncio
async def test_api_health():
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        response = await client.get("/api/v1/healthz")
        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "healthy"
        assert data["service"] == "omnirag-python-rag"


@pytest.mark.asyncio
async def test_document_upload_and_diffing_api():
    await init_database()
    test_file_name = f"guide_{uuid.uuid4().hex[:6]}.md"
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        # 1. Upload Document V1
        doc_v1_bytes = (
            b"# Engineering Guide\n\n"
            b"## Section 1: Architecture\n"
            b"We use a high-performance hybrid Go and Python architecture for extreme concurrency and AI flexibility.\n\n"
            b"## Section 2: Security\n"
            b"All API endpoints require strict tenant namespace isolation and dynamic ACL verification."
        )
        
        files_v1 = {"file": (test_file_name, io.BytesIO(doc_v1_bytes), "text/markdown")}
        headers = {"X-Workspace-ID": "ws_default"}

        resp_v1 = await client.post("/api/v1/documents/upload", files=files_v1, headers=headers)
        assert resp_v1.status_code == 200
        data_v1 = resp_v1.json()
        assert data_v1["status"] == "success"
        assert data_v1["total_chunks"] >= 2
        assert data_v1["version_number"] == 1
        assert data_v1["new_chunks_embedded"] >= 2

        doc_id = data_v1["document_id"]

        # 2. Upload Document V2 with only Section 2 modified
        doc_v2_bytes = (
            b"# Engineering Guide\n\n"
            b"## Section 1: Architecture\n"
            b"We use a high-performance hybrid Go and Python architecture for extreme concurrency and AI flexibility.\n\n"
            b"## Section 2: Security\n"
            b"All API endpoints require strict tenant namespace isolation and dynamic ACL verification with biometric auth."
        )
        
        files_v2 = {"file": (test_file_name, io.BytesIO(doc_v2_bytes), "text/markdown")}
        resp_v2 = await client.post("/api/v1/documents/upload", files=files_v2, headers=headers)
        assert resp_v2.status_code == 200
        data_v2 = resp_v2.json()
        assert data_v2["status"] == "success"
        assert data_v2["version_number"] == 2
        assert data_v2["reused_chunks_count"] >= 1
        assert data_v2["new_chunks_embedded"] == 1
        print(f"\n[API Upload Diff] Reused: {data_v2['reused_chunks_count']}, Embedded: {data_v2['new_chunks_embedded']}, Cost Savings: {data_v2['cost_savings_percent']}")

        # 3. List documents
        list_resp = await client.get("/api/v1/documents", headers=headers)
        assert list_resp.status_code == 200
        docs = list_resp.json()
        assert any(d["id"] == doc_id for d in docs)

        # 4. Get document versions
        ver_resp = await client.get(f"/api/v1/documents/{doc_id}/versions", headers=headers)
        assert ver_resp.status_code == 200
        versions = ver_resp.json()
        assert len(versions) == 2


@pytest.mark.asyncio
async def test_chat_streaming_api():
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        # Create conversation
        conv_resp = await client.post(
            "/api/v1/chat/conversations",
            json={"title": "Test RAG Chat"},
            headers={"X-Workspace-ID": "ws_default"},
        )
        assert conv_resp.status_code == 200
        conv = conv_resp.json()
        assert conv["title"] == "Test RAG Chat"

        # Stream chat completion
        chat_req = {
            "conversation_id": conv["id"],
            "messages": [{"role": "user", "content": "What is the architecture of the system?"}],
            "top_k": 5,
            "rerank_top_n": 3,
        }

        chat_resp = await client.post(
            "/api/v1/chat/completions/stream",
            json=chat_req,
            headers={"X-Workspace-ID": "ws_default"},
        )
        assert chat_resp.status_code == 200
        assert "text/event-stream" in chat_resp.headers["content-type"]
        
        # Read stream events
        body = chat_resp.text
        assert "citations" in body
        assert "token" in body
        assert "done" in body
        print("\n[Chat SSE Stream] Received events successfully with citations and tokens!")


@pytest.mark.asyncio
async def test_internal_embed_chunks_bridge():
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        payload = {
            "job_id": "job_123",
            "workspace_id": "ws_default",
            "document_id": "doc_bridge_test",
            "version_id": "v1",
            "namespace": "ws_ws_default",
            "file_name": "manual.txt",
            "source_uri": "s3://bucket/manual.txt",
            "chunks": [
                {
                    "id": "chunk_bridge_1",
                    "chunk_index": 0,
                    "chunk_hash": "hash_123",
                    "text_content": "OmniRAG provides distributed multi-tenant RAG scaling.",
                    "metadata": {"page": 1},
                }
            ],
        }

        resp = await client.post("/api/v1/internal/embed-chunks", json=payload)
        assert resp.status_code == 200
        assert resp.json()["status"] == "success"
        assert resp.json()["chunks_embedded"] == 1


@pytest.mark.asyncio
async def test_clerk_auth_and_session():
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        resp = await client.get("/api/v1/auth/me", headers={"X-Workspace-ID": "ws_enterprise_test"})
        assert resp.status_code == 200
        data = resp.json()
        assert data["workspace_id"] == "ws_enterprise_test"
        assert "user_id" in data


@pytest.mark.asyncio
async def test_document_deletion_lifecycle():
    await init_database()
    test_file_name = f"delete_test_{uuid.uuid4().hex[:6]}.md"
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        # Upload
        files = {"file": (test_file_name, io.BytesIO(b"# Deletion Test\n\nContent to be purged."), "text/markdown")}
        headers = {"X-Workspace-ID": "ws_default"}
        up_resp = await client.post("/api/v1/documents/upload", files=files, headers=headers)
        assert up_resp.status_code == 200
        doc_id = up_resp.json()["document_id"]

        # Delete
        del_resp = await client.delete(f"/api/v1/documents/{doc_id}", headers=headers)
        assert del_resp.status_code == 200
        del_data = del_resp.json()
        assert del_data["status"] == "success"
        assert del_data["document_id"] == doc_id
        assert del_data["chunks_deleted"] >= 1

        # Verify not found
        get_resp = await client.get("/api/v1/documents", headers=headers)
        assert get_resp.status_code == 200
        docs = get_resp.json()
        assert not any(d["id"] == doc_id for d in docs)


@pytest.mark.asyncio
async def test_settings_and_security_api():
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        headers = {"X-Workspace-ID": "ws_default"}
        # 1. Get settings
        get_res = await client.get("/api/v1/settings", headers=headers)
        assert get_res.status_code == 200
        settings_data = get_res.json()
        assert "embedding_provider" in settings_data
        assert "llm_provider" in settings_data

        # 2. Update settings
        update_req = {
            "embedding_provider": "fastembed",
            "llm_provider": "groq",
            "vector_store_provider": "mock",
        }
        post_res = await client.post("/api/v1/settings", json=update_req, headers=headers)
        assert post_res.status_code == 200
        assert post_res.json()["status"] == "success"
        assert post_res.json()["active_embedding_provider"] == "fastembed"
