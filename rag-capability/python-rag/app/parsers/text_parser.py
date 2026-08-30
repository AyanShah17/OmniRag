import re
from typing import List
from bs4 import BeautifulSoup
from app.parsers.base import BaseParser, ParsedDocument, ParsedSection


class TextParser(BaseParser):
    def __init__(self, mime_type: str = "text/plain"):
        self.mime_type = mime_type

    def parse(self, file_bytes: bytes, file_name: str) -> ParsedDocument:
        try:
            text = file_bytes.decode("utf-8")
        except UnicodeDecodeError:
            text = file_bytes.decode("latin-1", errors="replace")

        sections: List[ParsedSection] = []

        if self.mime_type == "text/html" or file_name.endswith((".html", ".htm")):
            soup = BeautifulSoup(text, "html.parser")
            # Remove scripts and styles
            for script in soup(["script", "style", "nav", "footer"]):
                script.decompose()
            clean_text = soup.get_text(separator="\n")
            lines = [line.strip() for line in clean_text.splitlines() if line.strip()]
            full_text = "\n".join(lines)
            sections.append(ParsedSection(text=full_text, heading="Web Content"))
            mime = "text/html"

        elif self.mime_type == "text/markdown" or file_name.endswith((".md", ".markdown")):
            # Split markdown by headings
            heading_regex = re.compile(r"^(#{1,6})\s+(.*)$", re.MULTILINE)
            chunks = heading_regex.split(text)
            
            full_text = text
            if len(chunks) > 1:
                # chunks = [preamble, '#', 'Heading 1', 'Content...', '##', 'Heading 2', ...]
                current_heading = "Overview"
                i = 0
                while i < len(chunks):
                    if chunks[i].startswith("#"):
                        current_heading = chunks[i+1].strip()
                        content = chunks[i+2].strip() if i+2 < len(chunks) else ""
                        if content:
                            sections.append(ParsedSection(text=content, heading=current_heading))
                        i += 3
                    else:
                        preamble = chunks[i].strip()
                        if preamble:
                            sections.append(ParsedSection(text=preamble, heading="Overview"))
                        i += 1
            else:
                sections.append(ParsedSection(text=text.strip(), heading="Overview"))
            mime = "text/markdown"

        else:
            # Plain text / CSV / JSON
            full_text = text
            paragraphs = [p.strip() for p in text.split("\n\n") if p.strip()]
            for p in paragraphs:
                sections.append(ParsedSection(text=p, heading="Section"))
            mime = "text/plain"

        return ParsedDocument(
            file_name=file_name,
            file_type=mime,
            full_text=full_text,
            sections=sections,
            page_count=1,
        )
