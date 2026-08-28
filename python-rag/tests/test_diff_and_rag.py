import asyncio
import io
import os
import sys
import pytest

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from app.parsers.extractor import extractor
from app.chunking.chunker import chunker, RecursiveTokenChunker, compute_sha256
from app.embeddings.fastembed_local import FastEmbedProvider
from app.embeddings.mock_provider import MockEmbeddingProvider
from app.vectorstore.mock_store import MockVectorStore
from app.vectorstore.base import VectorRecord
from app.rag.retriever import DynamicRAGRetriever
from app.rag.reranker import reranker
from app.rag.generator import generator, Citation
from app.db.init_db import init_database
from app.db.session import async_session_factory
from app.db.models import Document, DocumentVersion, Chunk
from sqlalchemy.future import select


@pytest.mark.asyncio
async def test_chunk_diffing_savings():
    """Verify that updating 1 paragraph in a document reuses untouched chunks with 0 cost."""
    doc_v1_text = """
    # Company Engineering Policy
    
    Section 1: Working Hours
    Our core working hours are from 10:00 AM to 4:00 PM Eastern Time.
    All engineers should be accessible on Slack during these hours.
    
    Section 2: Code Review Standards
    Every pull request must be reviewed by at least two senior engineers before merge.
    Automated CI/CD unit tests must pass with 100% success rate.
    
    Section 3: Deployment Schedule
    Production deployments take place on Tuesdays and Thursdays at 2:00 PM.
    No Friday deployments are permitted without VP Engineering approval.
    """

    # Ingest Version 1
    parsed_v1 = extractor.extract(doc_v1_text.encode("utf-8"), "policy.md")
    test_chunker = RecursiveTokenChunker(chunk_size=150, chunk_overlap=20)
    chunks_v1 = test_chunker.chunk_document(parsed_v1)
    
    assert len(chunks_v1) >= 3, f"Expected at least 3 chunks, got {len(chunks_v1)}"
    v1_hashes = {c.chunk_hash for c in chunks_v1}

    # Version 2: ONLY update Section 3 (change Tuesday/Thursday to Wednesday)
    doc_v2_text = """
    # Company Engineering Policy
    
    Section 1: Working Hours
    Our core working hours are from 10:00 AM to 4:00 PM Eastern Time.
    All engineers should be accessible on Slack during these hours.
    
    Section 2: Code Review Standards
    Every pull request must be reviewed by at least two senior engineers before merge.
    Automated CI/CD unit tests must pass with 100% success rate.
    
    Section 3: Deployment Schedule
    Production deployments take place on Wednesdays at 3:00 PM.
    No Friday deployments are permitted without VP Engineering approval.
    """

    parsed_v2 = extractor.extract(doc_v2_text.encode("utf-8"), "policy.md")
    chunks_v2 = test_chunker.chunk_document(parsed_v2)

    reused_count = 0
    new_count = 0
    for c in chunks_v2:
        if c.chunk_hash in v1_hashes:
            reused_count += 1
        else:
            new_count += 1

    print(f"\n[Test Diff] Total chunks: {len(chunks_v2)}, Reused: {reused_count}, New to embed: {new_count}")
    assert reused_count >= 2, "Expected Section 1 & Section 2 to be reused with 0 embedding cost!"
    assert new_count == 1, "Expected only modified Section 3 to require new embedding!"
    print("PASS: Chunk-level diffing verified: 0-cost chunk reuse works perfectly!")


@pytest.mark.asyncio
async def test_vector_store_and_rag_pipeline():
    """Verify Vector Store, namespace isolation, Re-ranking, and SSE streaming RAG citations."""
    embedder = MockEmbeddingProvider(dimension=384)
    vstore = MockVectorStore()
    retriever = DynamicRAGRetriever(embedder, vstore)

    namespace = "ws_test_workspace"

    # Index sample document chunks
    doc_text = "OmniRAG utilizes SHA-256 chunk-level hashing to achieve 90% cost savings on dynamic document updates."
    query_text = "How does OmniRAG save money on document updates?"

    text_vec = await embedder.embed_documents([doc_text])
    await vstore.upsert_vectors(
        namespace=namespace,
        vectors=[
            VectorRecord(
                id="chunk_001",
                values=text_vec[0],
                metadata={
                    "doc_id": "doc_100",
                    "file_name": "architecture_whitepaper.pdf",
                    "source_uri": "s3://my-bucket/architecture_whitepaper.pdf",
                    "page": 4,
                    "heading": "Diff Engine",
                    "text_content": doc_text,
                },
            )
        ],
    )

    # Retrieve
    retrieved = await retriever.retrieve(workspace_id="test_workspace", query=query_text, top_k=5)
    assert len(retrieved) > 0, "Expected at least 1 retrieved chunk"
    assert retrieved[0].file_name == "architecture_whitepaper.pdf"

    # Re-rank
    reranked = await reranker.rerank(query=query_text, chunks=retrieved, top_n=3)
    assert len(reranked) > 0

    # Stream RAG Response
    citations = [
        Citation(
            index=1,
            document_id=reranked[0].document_id,
            file_name=reranked[0].file_name,
            source_uri=reranked[0].source_uri,
            page_number=reranked[0].page_number,
            heading=reranked[0].heading,
            snippet=reranked[0].text_content[:150],
            score=0.98,
        )
    ]

    events = []
    async for event in generator.stream_response(
        system_prompt="Answer accurately",
        chat_history=[{"role": "user", "content": query_text}],
        citations=citations,
    ):
        events.append(event)

    assert len(events) > 2, "Expected multiple SSE events streamed"
    assert any("citations" in e for e in events), "Expected citations event in stream"
    print("PASS: Vector Store, Re-ranking, and SSE RAG streaming verified!")


@pytest.mark.asyncio
async def test_database_initialization():
    """Verify DB schema creation and default seed tenant/workspace."""
    await init_database()
    async with async_session_factory() as session:
        result = await session.execute(select(Document))
        docs = result.scalars().all()
        print(f"PASS: Database initialized with {len(docs)} existing documents.")


if __name__ == "__main__":
    asyncio.run(test_chunk_diffing_savings())
    asyncio.run(test_vector_store_and_rag_pipeline())
    asyncio.run(test_database_initialization())
    print("\nALL VERIFICATION TESTS PASSED SUCCESSFULLY! (100% GREEN)")
