import logging
from typing import List, Dict, Any, Optional
from pydantic import BaseModel, Field
from app.embeddings.base import BaseEmbeddingProvider
from app.vectorstore.base import BaseVectorStore

logger = logging.getLogger("omnirag.rag.retriever")


class RetrievedChunk(BaseModel):
    chunk_id: str
    document_id: str
    version_id: Optional[str] = None
    file_name: str
    source_uri: str
    text_content: str
    score: float
    page_number: Optional[int] = None
    heading: Optional[str] = None
    metadata: Dict[str, Any] = Field(default_factory=dict)


# Role granted to chunks that carry no explicit ACL metadata (e.g. legacy
# vectors upserted before ACL support existed) and to callers who present no
# roles at all. Keeping this narrow (rather than "grant everything") means an
# empty/missing acl_roles list can never widen access — it can only ever
# retrieve chunks nobody restricted, which is the fail-closed behavior we want.
DEFAULT_ACL_ROLE = "default"


class DynamicRAGRetriever:
    def __init__(self, embedding_provider: BaseEmbeddingProvider, vector_store: BaseVectorStore):
        self.embedding_provider = embedding_provider
        self.vector_store = vector_store

    async def retrieve(
        self,
        workspace_id: str,
        query: str,
        top_k: int = 10,
        acl_roles: Optional[List[str]] = None,
        filter_metadata: Optional[Dict[str, Any]] = None,
    ) -> List[RetrievedChunk]:
        namespace = f"ws_{workspace_id}"
        query_vec = await self.embedding_provider.embed_query(query)

        filter_dict: Dict[str, Any] = {}
        if filter_metadata:
            filter_dict.update(filter_metadata)

        # ACL enforcement at retrieval time: only chunks whose acl_roles list
        # intersects the caller's roles are eligible to be retrieved at all.
        # This runs unconditionally (never opt-in) so a caller cannot bypass it
        # by simply omitting acl_roles from the request — that instead narrows
        # them to DEFAULT_ACL_ROLE, the least-privileged case.
        effective_roles = list(acl_roles) if acl_roles else [DEFAULT_ACL_ROLE]
        filter_dict["acl_roles"] = {"$in": effective_roles}

        search_results = await self.vector_store.query_vectors(
            namespace=namespace,
            query_vector=query_vec,
            top_k=top_k,
            filter_metadata=filter_dict,
        )

        retrieved: List[RetrievedChunk] = []
        for r in search_results:
            meta = r.metadata
            retrieved.append(
                RetrievedChunk(
                    chunk_id=r.id,
                    document_id=meta.get("doc_id", meta.get("document_id", "")),
                    version_id=meta.get("version_id", ""),
                    file_name=meta.get("file_name", "Unknown Document"),
                    source_uri=meta.get("source_uri", ""),
                    text_content=meta.get("text_content", meta.get("text", "")),
                    score=r.score,
                    page_number=meta.get("page"),
                    heading=meta.get("heading"),
                    metadata=meta,
                )
            )

        logger.info(f"Retrieved {len(retrieved)} candidates for query in namespace '{namespace}'")
        return retrieved
