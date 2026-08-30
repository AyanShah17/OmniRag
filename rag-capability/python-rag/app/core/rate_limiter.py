import asyncio
import time
from collections import defaultdict, deque
from typing import Deque, Dict


class RateLimiter:
    def __init__(self, max_keys: int = 10_000) -> None:
        self._requests: Dict[str, Deque[float]] = defaultdict(deque)
        self._lock = asyncio.Lock()
        self._max_keys = max_keys

    async def allow(self, key: str, limit: int, window_seconds: int) -> bool:
        now = time.monotonic()
        cutoff = now - window_seconds
        async with self._lock:
            if key not in self._requests and len(self._requests) >= self._max_keys:
                stale_keys = [
                    item_key
                    for item_key, values in self._requests.items()
                    if not values or values[-1] <= cutoff
                ]
                for stale_key in stale_keys:
                    self._requests.pop(stale_key, None)
                if len(self._requests) >= self._max_keys:
                    return False
            timestamps = self._requests[key]
            while timestamps and timestamps[0] <= cutoff:
                timestamps.popleft()
            if len(timestamps) >= limit:
                return False
            timestamps.append(now)
            return True


rate_limiter = RateLimiter()
