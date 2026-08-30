import hashlib
import re
from typing import List, Dict, Any, Optional
from pydantic import BaseModel, Field
from app.parsers.base import ParsedDocument


class ChunkData(BaseModel):
    chunk_index: int
    text_content: str
    token_count: int
    chunk_hash: str
    metadata: Dict[str, Any] = Field(default_factory=dict)


def compute_sha256(text: str) -> str:
    # Normalize text to avoid whitespace mismatches
    normalized = "\n".join(line.strip() for line in text.splitlines() if line.strip())
    return hashlib.sha256(normalized.encode("utf-8")).hexdigest()


class RecursiveTokenChunker:
    def __init__(
        self,
        chunk_size: int = 500,       # Approximate target token/char length
        chunk_overlap: int = 50,
        separators: Optional[List[str]] = None,
    ):
        self.chunk_size = chunk_size
        self.chunk_overlap = chunk_overlap
        self.separators = separators or ["\n\n", "\n", ". ", "! ", "? ", "; ", " ", ""]

    def chunk_document(self, parsed_doc: ParsedDocument) -> List[ChunkData]:
        chunks: List[ChunkData] = []
        global_index = 0

        # If document has structured sections (e.g. from PDF pages or DOCX headings)
        if parsed_doc.sections:
            for section in parsed_doc.sections:
                section_text = section.text.strip()
                if not section_text:
                    continue

                split_texts = self._split_text(section_text, self.chunk_size, self.chunk_overlap)
                for t in split_texts:
                    t_clean = t.strip()
                    if not t_clean:
                        continue

                    meta = dict(section.metadata)
                    if section.page_number:
                        meta["page"] = section.page_number
                    if section.heading:
                        meta["heading"] = section.heading
                    meta["file_name"] = parsed_doc.file_name

                    token_count = max(1, len(t_clean.split()))
                    chunk_hash = compute_sha256(t_clean)

                    chunks.append(
                        ChunkData(
                            chunk_index=global_index,
                            text_content=t_clean,
                            token_count=token_count,
                            chunk_hash=chunk_hash,
                            metadata=meta,
                        )
                    )
                    global_index += 1
        else:
            # Fallback to full text splitting
            split_texts = self._split_text(parsed_doc.full_text, self.chunk_size, self.chunk_overlap)
            for t in split_texts:
                t_clean = t.strip()
                if not t_clean:
                    continue
                token_count = max(1, len(t_clean.split()))
                chunk_hash = compute_sha256(t_clean)
                chunks.append(
                    ChunkData(
                        chunk_index=global_index,
                        text_content=t_clean,
                        token_count=token_count,
                        chunk_hash=chunk_hash,
                        metadata={"file_name": parsed_doc.file_name},
                    )
                )
                global_index += 1

        return chunks

    def _split_text(self, text: str, chunk_size: int, chunk_overlap: int) -> List[str]:
        if len(text) <= chunk_size:
            return [text]

        # Try splitting on separators recursively
        for sep in self.separators:
            if not sep:
                # Last resort: slice by length
                return [text[i : i + chunk_size] for i in range(0, len(text), chunk_size - chunk_overlap)]

            splits = text.split(sep)
            if len(splits) <= 1:
                continue

            chunks: List[str] = []
            current = []
            current_len = 0

            for part in splits:
                part_len = len(part) + len(sep)
                if current_len + part_len > chunk_size and current:
                    combined = sep.join(current)
                    chunks.append(combined)
                    # Keep overlap
                    overlap_parts = []
                    overlap_len = 0
                    for op in reversed(current):
                        if overlap_len + len(op) < chunk_overlap:
                            overlap_parts.insert(0, op)
                            overlap_len += len(op) + len(sep)
                        else:
                            break
                    current = overlap_parts
                    current_len = overlap_len

                current.append(part)
                current_len += part_len

            if current:
                chunks.append(sep.join(current))

            return chunks

        return [text]


chunker = RecursiveTokenChunker()
