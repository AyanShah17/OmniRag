import os
import sys

import pytest
from httpx import ASGITransport, AsyncClient
from sqlalchemy.future import select

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from app.chunking.chunker import RecursiveTokenChunker
from app.core.config import settings
from app.core.rate_limiter import RateLimiter
from app.db.init_db import init_database
from app.db.models import Document
from app.db.session import async_session_factory
from app.main import app
from app.parsers.extractor import DocumentExtractor
from app.services.deployment_settings import DeploymentSettingsStore
from app.services.document_ingestion import DocumentIndexingError, DocumentIngestionService


class FailingIndexer:
    async def index_payload(self, _payload):
        raise RuntimeError("vector store unavailable")

    async def compensate(self, _result):
        raise AssertionError("No completed vector write should be compensated")


@pytest.mark.asyncio
async def test_ingestion_rolls_back_relational_state_when_indexing_fails():
    await init_database()
    service = DocumentIngestionService(
        parser=DocumentExtractor(),
        chunker=RecursiveTokenChunker(chunk_size=100),
        indexer=FailingIndexer(),
    )
    external_id = "test/acid/vector-failure.txt"

    async with async_session_factory() as session:
        with pytest.raises(DocumentIndexingError):
            await service.ingest(
                file_bytes=b"A document that must not survive a failed vector write.",
                file_name="vector-failure.txt",
                content_type="text/plain",
                workspace_id="ws_default",
                external_id=external_id,
                db=session,
            )

    async with async_session_factory() as session:
        document = await session.scalar(select(Document).where(Document.external_id == external_id))
        assert document is None


@pytest.mark.asyncio
async def test_upload_limit_and_security_headers(monkeypatch):
    monkeypatch.setattr(settings, "MAX_UPLOAD_BYTES", 3)
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        response = await client.post(
            "/api/v1/documents/upload",
            headers={"X-Workspace-ID": "ws_default"},
            files={"file": ("too-large.txt", b"four", "text/plain")},
        )
        health = await client.get("/api/v1/healthz")

    assert response.status_code == 413
    assert health.headers["x-content-type-options"] == "nosniff"
    assert health.headers["x-frame-options"] == "DENY"


@pytest.mark.asyncio
async def test_rate_limiter_bounds_unique_keys():
    limiter = RateLimiter(max_keys=2)
    assert await limiter.allow("one", 1, 60)
    assert await limiter.allow("two", 1, 60)
    assert not await limiter.allow("three", 1, 60)


def test_deployment_settings_rejects_line_injection(tmp_path):
    store = DeploymentSettingsStore(str(tmp_path / "omnirag.env"))
    with pytest.raises(ValueError):
        store.update({"OPENAI_API_KEY": "valid\nAUTH_MODE=development"})
    assert not (tmp_path / "omnirag.env").exists()
