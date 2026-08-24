import os
from typing import Dict
from app.parsers.base import BaseParser, ParsedDocument
from app.parsers.pdf_parser import PDFParser
from app.parsers.docx_parser import DOCXParser
from app.parsers.text_parser import TextParser


class DocumentExtractor:
    def __init__(self):
        self._parsers: Dict[str, BaseParser] = {
            ".pdf": PDFParser(),
            ".docx": DOCXParser(),
            ".doc": DOCXParser(),
            ".md": TextParser(mime_type="text/markdown"),
            ".markdown": TextParser(mime_type="text/markdown"),
            ".html": TextParser(mime_type="text/html"),
            ".htm": TextParser(mime_type="text/html"),
            ".txt": TextParser(mime_type="text/plain"),
            ".csv": TextParser(mime_type="text/csv"),
            ".json": TextParser(mime_type="application/json"),
        }
        self._default_parser = TextParser()

    def extract(self, file_bytes: bytes, file_name: str, content_type: str = "") -> ParsedDocument:
        ext = os.path.splitext(file_name)[1].lower()
        parser = self._parsers.get(ext)
        if not parser:
            if "pdf" in content_type:
                parser = self._parsers[".pdf"]
            elif "word" in content_type:
                parser = self._parsers[".docx"]
            elif "html" in content_type:
                parser = self._parsers[".html"]
            else:
                parser = self._default_parser

        return parser.parse(file_bytes, file_name)


extractor = DocumentExtractor()
