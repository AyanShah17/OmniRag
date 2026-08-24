from abc import ABC, abstractmethod
from typing import List, Dict, Any, Optional
from pydantic import BaseModel


class ParsedSection(BaseModel):
    text: str
    page_number: Optional[int] = None
    heading: Optional[str] = None
    metadata: Dict[str, Any] = {}


class ParsedDocument(BaseModel):
    file_name: str
    file_type: str
    full_text: str
    sections: List[ParsedSection]
    metadata: Dict[str, Any] = {}
    page_count: int = 1


class BaseParser(ABC):
    @abstractmethod
    def parse(self, file_bytes: bytes, file_name: str) -> ParsedDocument:
        """Extract clean text and structural sections from raw file bytes."""
        pass
