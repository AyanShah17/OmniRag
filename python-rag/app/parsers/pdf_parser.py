import io
from typing import List
from pypdf import PdfReader
from app.parsers.base import BaseParser, ParsedDocument, ParsedSection


class PDFParser(BaseParser):
    def parse(self, file_bytes: bytes, file_name: str) -> ParsedDocument:
        stream = io.BytesIO(file_bytes)
        reader = PdfReader(stream)
        
        sections: List[ParsedSection] = []
        full_text_parts: List[str] = []

        total_pages = len(reader.pages)
        for page_idx, page in enumerate(reader.pages):
            text = page.extract_text() or ""
            text = text.strip()
            if text:
                full_text_parts.append(text)
                sections.append(
                    ParsedSection(
                        text=text,
                        page_number=page_idx + 1,
                        heading=f"Page {page_idx + 1}",
                        metadata={"page": page_idx + 1, "total_pages": total_pages},
                    )
                )

        full_text = "\n\n".join(full_text_parts)
        return ParsedDocument(
            file_name=file_name,
            file_type="application/pdf",
            full_text=full_text,
            sections=sections,
            page_count=total_pages,
            metadata={"total_pages": total_pages},
        )
