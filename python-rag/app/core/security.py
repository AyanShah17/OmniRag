import os
import base64
from typing import Optional
from cryptography.hazmat.primitives.ciphers.aead import AESGCM

# Default 32-byte encryption key derived or loaded from env
DEFAULT_KEY_HEX = os.getenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")


def _get_key_bytes(key_hex: Optional[str] = None) -> bytes:
    key_str = key_hex or os.getenv("ENCRYPTION_KEY", DEFAULT_KEY_HEX)
    try:
        raw = bytes.fromhex(key_str)
        if len(raw) == 32:
            return raw
    except Exception:
        pass
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
        # Graceful fallback: return as-is if raw unencrypted string
        return encrypted_b64


def mask_secret(secret: str) -> str:
    """Masks secrets for safe display in logs and UI."""
    if not secret:
        return ""
    if len(secret) <= 6:
        return "••••••"
    return secret[:4] + "••••••••" + secret[-2:]
