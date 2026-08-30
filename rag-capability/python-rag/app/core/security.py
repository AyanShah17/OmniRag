import os
import base64
from typing import Optional, Protocol
from cryptography.hazmat.primitives.ciphers.aead import AESGCM
from app.core.config import settings

DEVELOPMENT_KEY_HEX = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"


class SecretProvider(Protocol):
    def get_secret(self, name: str) -> Optional[str]: ...


class EnvironmentSecretProvider:
    def get_secret(self, name: str) -> Optional[str]:
        return os.getenv(name)


secret_provider: SecretProvider = EnvironmentSecretProvider()


def _get_key_bytes(key_hex: Optional[str] = None) -> bytes:
    key_str = key_hex or secret_provider.get_secret("ENCRYPTION_KEY") or settings.ENCRYPTION_KEY
    if not key_str:
        if settings.is_production_auth:
            raise RuntimeError("ENCRYPTION_KEY is required in production")
        key_str = DEVELOPMENT_KEY_HEX
    try:
        raw = bytes.fromhex(key_str)
        if len(raw) == 32:
            return raw
    except ValueError:
        raw = b""
    if settings.is_production_auth:
        raise RuntimeError("ENCRYPTION_KEY must be 64 hexadecimal characters")
    # Fallback to padded bytes if key is not hex
    return (key_str.encode("utf-8") + b"0" * 32)[:32]


def encrypt_data(plain_text: str, key_hex: Optional[str] = None) -> str:
    """Encrypts plain text using AES-256-GCM with a random 12-byte nonce."""
    if not plain_text:
        return ""
    key = _get_key_bytes(key_hex)
    aesgcm = AESGCM(key)
    nonce = os.urandom(12)
    ciphertext = aesgcm.encrypt(nonce, plain_text.encode("utf-8"), None)
    # Combine nonce + ciphertext
    payload = nonce + ciphertext
    return base64.b64encode(payload).decode("utf-8")


def decrypt_data(encrypted_b64: str, key_hex: Optional[str] = None) -> str:
    """Decrypts AES-256-GCM ciphertext."""
    if not encrypted_b64:
        return ""
    try:
        key = _get_key_bytes(key_hex)
        payload = base64.b64decode(encrypted_b64.encode("utf-8"))
        if len(payload) < 12:
            return encrypted_b64  # Return raw if not valid encrypted payload
        nonce = payload[:12]
        ciphertext = payload[12:]
        aesgcm = AESGCM(key)
        decrypted = aesgcm.decrypt(nonce, ciphertext, None)
        return decrypted.decode("utf-8")
    except Exception:
        if settings.is_production_auth:
            raise
        # Preserve compatibility with legacy plaintext values in development.
        return encrypted_b64


def mask_secret(secret: str) -> str:
    """Masks secrets for safe display in logs and UI."""
    if not secret:
        return ""
    if len(secret) <= 6:
        return "••••••"
    return secret[:4] + "••••••••" + secret[-2:]
