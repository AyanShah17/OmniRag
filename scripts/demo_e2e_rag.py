"""
OmniRAG - End-to-End Dynamic RAG Interactive CLI Demonstration
Demonstrates:
1. Document Ingestion & Chunking
2. Version 1 vs Version 2 Chunk-Level Diffing (Zero-Cost Re-linking)
3. Dynamic Vector Retrieval & Neural Re-ranking
4. Real-time Streaming RAG Chat with Interactive Grounded Citations
"""

import asyncio
import io
import json
import os
import sys
import time
import uuid

# Add python-rag to sys.path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../python-rag")))

from httpx import AsyncClient, ASGITransport
from app.main import app
from app.db.init_db import init_database


CYAN = "\033[96m"
GREEN = "\033[92m"
YELLOW = "\033[93m"
MAGENTA = "\033[95m"
BOLD = "\033[1m"
RESET = "\033[0m"


async def main():
    print(f"\n{BOLD}{CYAN}======================================================================{RESET}")
    print(f"{BOLD}{CYAN}      OmniRAG Dynamic Enterprise RAG SaaS - Live Demonstration        {RESET}")
    print(f"{BOLD}{CYAN}======================================================================{RESET}\n")

    await init_database()

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        # Step 1: Health Check
        print(f"{YELLOW}[1/4] Checking System Health & Active Providers...{RESET}")
        health_resp = await client.get("/api/v1/healthz")
        health = health_resp.json()
        print(f"      - Status:             {GREEN}{health['status']}{RESET}")
        print(f"      - Embedding Engine:   {health['embedding_provider']}")
        print(f"      - Vector Store:       {health['vector_store_provider']}")
        print(f"      - LLM Provider:       {health['llm_provider']}")
        print()

        # Step 2: Ingest Document Version 1
        print(f"{YELLOW}[2/4] Ingesting Document Version 1 (AWS S3 Cloud Policy)...{RESET}")
        v1_content = """# Enterprise Cloud & AI Security Guidelines

## Section 1: Data Retention & Encryption
All customer data stored in AWS S3 and Azure Blob Storage must be encrypted at rest using AES-256-GCM.
Data retention period for audit logs is strictly 365 days.

## Section 2: AI Model Governance
Only pre-approved LLM providers (OpenRouter, Groq, and Self-Hosted FastEmbed) are permitted for dynamic RAG.
No confidential internal code may be sent to public unauthenticated endpoints.

## Section 3: Incident Response Protocol
In the event of a suspected breach, the SecOps team must be notified within 15 minutes.
Production systems will automatically switch to quarantined namespace isolation mode.
"""
        doc_name = f"cloud_security_policy_{uuid.uuid4().hex[:6]}.md"
        files_v1 = {"file": (doc_name, io.BytesIO(v1_content.encode("utf-8")), "text/markdown")}
        headers = {"X-Workspace-ID": "ws_demo_enterprise"}

        v1_resp = await client.post("/api/v1/documents/upload", files=files_v1, headers=headers)
        res_v1 = v1_resp.json()
        doc_id = res_v1["document_id"]
        print(f"      - Document:           {doc_name}")
        print(f"      - Document ID:        {res_v1['document_id']}")
        print(f"      - Version:            {GREEN}v{res_v1['version_number']}{RESET}")
        print(f"      - Total Chunks:       {res_v1['total_chunks']}")
        print(f"      - Embedded Chunks:    {res_v1['new_chunks_embedded']}")
        print()

        # Step 3: Modify 1 Section and Ingest Version 2 (Showcase Chunk-Level Diffing)
        print(f"{YELLOW}[3/4] Updating Document: Modifying Section 3 ONLY (Simulating live doc edit)...{RESET}")
        v2_content = """# Enterprise Cloud & AI Security Guidelines

## Section 1: Data Retention & Encryption
All customer data stored in AWS S3 and Azure Blob Storage must be encrypted at rest using AES-256-GCM.
Data retention period for audit logs is strictly 365 days.

## Section 2: AI Model Governance
Only pre-approved LLM providers (OpenRouter, Groq, and Self-Hosted FastEmbed) are permitted for dynamic RAG.
No confidential internal code may be sent to public unauthenticated endpoints.

## Section 3: Incident Response Protocol
In the event of a suspected breach, the SecOps team must be notified within 5 minutes (updated from 15 mins).
Production systems will automatically switch to quarantined namespace isolation mode with real-time biometric lockout.
"""
        files_v2 = {"file": (doc_name, io.BytesIO(v2_content.encode("utf-8")), "text/markdown")}
        v2_resp = await client.post("/api/v1/documents/upload", files=files_v2, headers=headers)
        res_v2 = v2_resp.json()

        print(f"      - Version:            {GREEN}v{res_v2['version_number']}{RESET}")
        print(f"      - Total Chunks:       {res_v2['total_chunks']}")
        print(f"      - Reused Chunks:      {GREEN}{res_v2['reused_chunks_count']} (Untouched chunks re-linked at $0 cost!){RESET}")
        print(f"      - Newly Embedded:     {YELLOW}{res_v2['new_chunks_embedded']} (Only modified Section 3 was embedded){RESET}")
        print(f"      - Cost Savings:       {BOLD}{GREEN}{res_v2['cost_savings_percent']}{RESET}")
        print()

        # Step 4: Streaming Dynamic RAG Query
        user_query = "What is our incident response SLA and what encryption is required for S3?"
        print(f"{YELLOW}[4/4] Executing Real-Time Streaming Dynamic RAG Query...{RESET}")
        print(f"      {BOLD}User Query:{RESET} \"{user_query}\"\n")

        chat_payload = {
            "messages": [{"role": "user", "content": user_query}],
            "top_k": 5,
            "rerank_top_n": 3,
        }

        print(f"{MAGENTA}--------------------------------- AI RESPONSE STREAM ---------------------------------{RESET}")
        
        chat_resp = await client.post("/api/v1/chat/completions/stream", json=chat_payload, headers=headers)
        citations_received = []

        for line in chat_resp.text.split("\n\n"):
            if not line.startswith("data: "):
                continue
            data_str = line[6:].strip()
            if not data_str:
                continue
            try:
                event_obj = json.loads(data_str)
                event_type = event_obj.get("event")

                if event_type == "citations":
                    citations_received = event_obj.get("data", [])
                elif event_type == "token":
                    sys.stdout.write(event_obj.get("data", ""))
                    sys.stdout.flush()
                elif event_type == "done":
                    break
            except Exception:
                continue

        print(f"\n{MAGENTA}--------------------------------------------------------------------------------------{RESET}\n")

        if citations_received:
            print(f"{BOLD}{GREEN}Interactive Source Citations Grounding:{RESET}")
            for c in citations_received:
                page_info = f" | Page: {c['page_number']}" if c.get("page_number") else ""
                section_info = f" | Section: {c['heading']}" if c.get("heading") else ""
                print(f"  [{c['index']}] {BOLD}{c['file_name']}{RESET}{section_info}{page_info} (Relevance Score: {c['score']})")
                print(f"      \"{c['snippet']}\"")
                print()

    print(f"{BOLD}{GREEN}[SUCCESS] Demonstration Completed Successfully!{RESET}\n")


if __name__ == "__main__":
    asyncio.run(main())
