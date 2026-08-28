import json
import os
import tempfile
from pathlib import Path
from typing import Mapping


class DeploymentSettingsStore:
    """Atomically persists deployment settings with restrictive permissions."""

    def __init__(self, path: str) -> None:
        self._path = Path(path)

    def update(self, updates: Mapping[str, str]) -> None:
        if not updates:
            return
        for key, value in updates.items():
            if not key.replace("_", "").isalnum() or not key.isupper():
                raise ValueError(f"Invalid settings key: {key}")
            if any(character in value for character in ("\r", "\n", "\x00")):
                raise ValueError(f"Invalid control character in setting: {key}")

        existing = self._path.read_text(encoding="utf-8").splitlines(keepends=True) if self._path.exists() else []
        written_keys = set()
        output = []
        for line in existing:
            stripped = line.strip()
            if "=" in line and not stripped.startswith("#"):
                key = line.split("=", 1)[0].strip()
                if key in updates:
                    output.append(f"{key}={json.dumps(updates[key])}\n")
                    written_keys.add(key)
                    continue
            output.append(line)
        for key, value in updates.items():
            if key not in written_keys:
                output.append(f"{key}={json.dumps(value)}\n")

        self._path.parent.mkdir(parents=True, exist_ok=True)
        temporary_path = None
        try:
            with tempfile.NamedTemporaryFile(
                mode="w",
                encoding="utf-8",
                dir=self._path.parent,
                prefix=f".{self._path.name}.",
                suffix=".tmp",
                delete=False,
            ) as temporary:
                temporary_path = Path(temporary.name)
                temporary.writelines(output)
                temporary.flush()
                os.fsync(temporary.fileno())
            os.chmod(temporary_path, 0o600)
            os.replace(temporary_path, self._path)
        finally:
            if temporary_path is not None and temporary_path.exists():
                temporary_path.unlink()
