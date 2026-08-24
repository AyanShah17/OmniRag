import logging
from typing import List, Dict, Any, AsyncGenerator, Optional
from app.embeddings.factory import get_embedding_provider
from app.vectorstore.factory import get_vector_store
from app.rag.retriever import DynamicRAGRetriever
from app.rag.reranker import reranker
from app.rag.generator import generator, Citation

logger = logging.getLogger("omnirag.rag.pipeline")


class RAGPipeline:
    def __init__(self):
        self.embedding_provider = get_embedding_provider()
        self.vector_store = get_vector_store()
        self.retriever = DynamicRAGRetriever(self.embedding_provider, self.vector_store)

    async def execute_rag_stream(
        self,
        workspace_id: str,
        chat_history: List[Dict[str, str]],
        top_k: int = 10,
        rerank_top_n: int = 4,
        acl_roles: Optional[List[str]] = None,
        filter_metadata: Optional[Dict[str, Any]] = None,
    ) -> AsyncGenerator[str, None]:
        if not chat_history:
            return

        last_user_message = chat_history[-1]["content"]

        # 1. Vector Retrieval
        retrieved_chunks = await self.retriever.retrieve(
            workspace_id=workspace_id,
            query=last_user_message,
            top_k=top_k,
            acl_roles=acl_roles,
            filter_metadata=filter_metadata,
        )

        # 2. Semantic Re-ranking
        reranked_chunks = await reranker.rerank(
            query=last_user_message,
            chunks=retrieved_chunks,
            top_n=rerank_top_n,
        )

        # 3. Assemble Grounded Citations
        citations: List[Citation] = []
        context_passages: List[str] = []

        for idx, chunk in enumerate(reranked_chunks):
            citation_idx = idx + 1
            snippet = chunk.text_content[:250].strip() + ("..." if len(chunk.text_content) > 250 else "")
            
            citations.append(
                Citation(
                    index=citation_idx,
                    document_id=chunk.document_id,
                    version_id=chunk.version_id,
                    file_name=chunk.file_name,
                    source_uri=chunk.source_uri,
                    page_number=chunk.page_number,
                    heading=chunk.heading,
                    snippet=snippet,
                    score=round(chunk.score, 4),
                )
            )

            passage_header = f"[{citation_idx}] Source: {chunk.file_name}"
            if chunk.page_number:
                passage_header += f" | Page: {chunk.page_number}"
            if chunk.heading:
                passage_header += f" | Section: {chunk.heading}"

            context_passages.append(f"{passage_header}\nContent:\n{chunk.text_content}")

        context_block = "\n\n---\n\n".join(context_passages)

        # 4. System Prompt Synthesis
        system_prompt = f"""You are OmniRAG, a precision enterprise AI assistant.
Answer the user's inquiry based exclusively and accurately on the provided knowledge base context below.

CRITICAL INSTRUCTIONS:
1. Always attribute facts and statements to their source citation using brackets, e.g. [1], [2].
2. If the answer cannot be determined from the provided context, state clearly that the information is not present in the connected documents.
3. Be concise, professional, and well-structured.

KNOWLEDGE BASE CONTEXT:
{context_block}
"""

        # 5. Stream LLM Response & Citations
        async for chunk in generator.stream_response(system_prompt, chat_history, citations):
            yield chunk


rag_pipeline = RAGPipeline()
