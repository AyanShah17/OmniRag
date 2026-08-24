import asyncio
import json
import logging
from typing import List, Dict, Any, AsyncGenerator, Optional
import httpx
from pydantic import BaseModel
from app.core.config import settings

logger = logging.getLogger("omnirag.rag.generator")


class Citation(BaseModel):
    index: int
    document_id: str
    version_id: Optional[str] = None
    file_name: str
    source_uri: str
    page_number: Optional[int] = None
    heading: Optional[str] = None
    snippet: str
    score: float


class LLMGenerator:
    def __init__(self):
        self.provider = settings.LLM_PROVIDER.lower()

    async def stream_response(
        self,
        system_prompt: str,
        chat_history: List[Dict[str, str]],
        citations: List[Citation],
    ) -> AsyncGenerator[str, None]:
        """Streams response tokens and citation metadata over SSE protocol."""
        # First event: emit citation metadata payload
        citation_payload = {
            "event": "citations",
            "data": [c.model_dump() for c in citations],
        }
        yield f"data: {json.dumps(citation_payload)}\n\n"

        if self.provider == "openrouter" and settings.OPENROUTER_API_KEY:
            async for token in self._stream_openrouter(system_prompt, chat_history):
                yield f"data: {json.dumps({'event': 'token', 'data': token})}\n\n"
        elif self.provider == "groq" and settings.GROQ_API_KEY:
            async for token in self._stream_groq(system_prompt, chat_history):
                yield f"data: {json.dumps({'event': 'token', 'data': token})}\n\n"
        elif self.provider == "openai" and settings.OPENAI_API_KEY:
            async for token in self._stream_openai(system_prompt, chat_history):
                yield f"data: {json.dumps({'event': 'token', 'data': token})}\n\n"
        else:
            # High-fidelity Mock/Local generator for instant local zero-cost verification
            async for token in self._stream_mock(system_prompt, chat_history, citations):
                yield f"data: {json.dumps({'event': 'token', 'data': token})}\n\n"

        # Final event: emit completion marker
        yield f"data: {json.dumps({'event': 'done'})}\n\n"

    async def _stream_openrouter(self, system_prompt: str, chat_history: List[Dict[str, str]]) -> AsyncGenerator[str, None]:
        url = "https://openrouter.ai/api/v1/chat/completions"
        headers = {
            "Authorization": f"Bearer {settings.OPENROUTER_API_KEY}",
            "Content-Type": "application/json",
            "HTTP-Referer": "https://omnirag.ai",
            "X-Title": "OmniRAG",
        }
        messages = [{"role": "system", "content": system_prompt}] + chat_history
        payload = {
            "model": settings.OPENROUTER_MODEL,
            "messages": messages,
            "stream": True,
            "temperature": 0.2,
        }

        async with httpx.AsyncClient(timeout=60.0) as client:
            async with client.stream("POST", url, json=payload, headers=headers) as resp:
                async for line in resp.aiter_lines():
                    if line.startswith("data: "):
                        data_str = line[6:].strip()
                        if data_str == "[DONE]":
                            break
                        try:
                            data = json.loads(data_str)
                            delta = data["choices"][0]["delta"].get("content", "")
                            if delta:
                                yield delta
                        except Exception:
                            continue

    async def _stream_groq(self, system_prompt: str, chat_history: List[Dict[str, str]]) -> AsyncGenerator[str, None]:
        url = "https://api.groq.com/openai/v1/chat/completions"
        headers = {
            "Authorization": f"Bearer {settings.GROQ_API_KEY}",
            "Content-Type": "application/json",
        }
        messages = [{"role": "system", "content": system_prompt}] + chat_history
        payload = {
            "model": settings.GROQ_MODEL,
            "messages": messages,
            "stream": True,
            "temperature": 0.2,
        }

        async with httpx.AsyncClient(timeout=60.0) as client:
            async with client.stream("POST", url, json=payload, headers=headers) as resp:
                async for line in resp.aiter_lines():
                    if line.startswith("data: "):
                        data_str = line[6:].strip()
                        if data_str == "[DONE]":
                            break
                        try:
                            data = json.loads(data_str)
                            delta = data["choices"][0]["delta"].get("content", "")
                            if delta:
                                yield delta
                        except Exception:
                            continue

    async def _stream_openai(self, system_prompt: str, chat_history: List[Dict[str, str]]) -> AsyncGenerator[str, None]:
        url = "https://api.openai.com/v1/chat/completions"
        headers = {
            "Authorization": f"Bearer {settings.OPENAI_API_KEY}",
            "Content-Type": "application/json",
        }
        messages = [{"role": "system", "content": system_prompt}] + chat_history
        payload = {
            "model": settings.OPENAI_MODEL,
            "messages": messages,
            "stream": True,
            "temperature": 0.2,
        }

        async with httpx.AsyncClient(timeout=60.0) as client:
            async with client.stream("POST", url, json=payload, headers=headers) as resp:
                async for line in resp.aiter_lines():
                    if line.startswith("data: "):
                        data_str = line[6:].strip()
                        if data_str == "[DONE]":
                            break
                        try:
                            data = json.loads(data_str)
                            delta = data["choices"][0]["delta"].get("content", "")
                            if delta:
                                yield delta
                        except Exception:
                            continue

    async def _stream_mock(
        self,
        system_prompt: str,
        chat_history: List[Dict[str, str]],
        citations: List[Citation],
    ) -> AsyncGenerator[str, None]:
        """Generates grounded answer tokens referencing retrieved citations."""
        last_query = chat_history[-1]["content"] if chat_history else "information"
        
        if citations:
            intro = f"Based on the connected knowledge base documents, here is the answer regarding '{last_query}':\n\n"
            for word in intro.split(" "):
                yield word + " "
                await asyncio.sleep(0.015)

            for c in citations:
                point = f"• According to **{c.file_name}**"
                if c.page_number:
                    point += f" (Page {c.page_number})"
                point += f": \"{c.snippet}\" [{c.index}]\n\n"

                for word in point.split(" "):
                    yield word + " "
                    await asyncio.sleep(0.015)

            conclusion = "Let me know if you would like me to dive deeper into any specific document or section!"
            for word in conclusion.split(" "):
                yield word + " "
                await asyncio.sleep(0.015)
        else:
            msg = f"I scanned the knowledge base for '{last_query}', but could not find relevant context in the permitted documents. Please check that the connector is active and indexed."
            for word in msg.split(" "):
                yield word + " "
                await asyncio.sleep(0.015)


generator = LLMGenerator()
