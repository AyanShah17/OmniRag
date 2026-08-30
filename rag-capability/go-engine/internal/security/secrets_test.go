package security

import "testing"

func TestConnectorConfigEncryptionRoundTrip(t *testing.T) {
	config := map[string]interface{}{
		"bucket":            "documents",
		"secret_access_key": "sensitive",
	}
	encrypted, err := EncryptConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	if encrypted["secret_access_key"] == "sensitive" {
		t.Fatal("secret was stored as plaintext")
	}
	decrypted, err := DecryptConfig(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted["secret_access_key"] != "sensitive" || decrypted["bucket"] != "documents" {
		t.Fatalf("unexpected decrypted config: %#v", decrypted)
	}
	if MaskConfig(encrypted)["secret_access_key"] != "********" {
		t.Fatal("secret was not masked")
	}
}
