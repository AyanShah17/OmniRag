package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	envelopePrefix    = "enc:v1:"
	developmentKeyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

func encryptionKey() ([]byte, error) {
	keyHex := os.Getenv("ENCRYPTION_KEY")
	if keyHex == "" {
		if strings.EqualFold(os.Getenv("AUTH_MODE"), "production") {
			return nil, fmt.Errorf("ENCRYPTION_KEY is required in production")
		}
		keyHex = developmentKeyHex
	}
	key, err := hex.DecodeString(keyHex)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be 64 hexadecimal characters")
	}
	return key, nil
}

func isSecretField(name string) bool {
	normalized := strings.ToLower(name)
	return strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "api_key") ||
		strings.Contains(normalized, "access_key") ||
		strings.Contains(normalized, "connection_string") ||
		strings.Contains(normalized, "service_role")
}

func EncryptConfig(config map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(config))
	for name, value := range config {
		plain, ok := value.(string)
		if !ok || plain == "" || !isSecretField(name) {
			result[name] = value
			continue
		}
		encrypted, err := encryptString(plain)
		if err != nil {
			return nil, err
		}
		result[name] = encrypted
	}
	return result, nil
}

func DecryptConfig(config map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(config))
	for name, value := range config {
		encrypted, ok := value.(string)
		if !ok || !strings.HasPrefix(encrypted, envelopePrefix) {
			result[name] = value
			continue
		}
		plain, err := decryptString(encrypted)
		if err != nil {
			return nil, fmt.Errorf("decrypt connector field %q: %w", name, err)
		}
		result[name] = plain
	}
	return result, nil
}

func MaskConfig(config map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(config))
	for name, value := range config {
		if isSecretField(name) {
			result[name] = "********"
		} else {
			result[name] = value
		}
	}
	return result
}

func encryptString(plain string) (string, error) {
	key, err := encryptionKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return envelopePrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decryptString(envelope string) (string, error) {
	payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(envelope, envelopePrefix))
	if err != nil {
		return "", err
	}
	key, err := encryptionKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(payload) < gcm.NonceSize() {
		return "", fmt.Errorf("encrypted payload is truncated")
	}
	nonce, ciphertext := payload[:gcm.NonceSize()], payload[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
