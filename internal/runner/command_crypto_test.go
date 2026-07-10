package runner

import (
	"strings"
	"testing"
)

func TestEncryptCommandStringRoundTrip(t *testing.T) {
	plaintext := "git checkout -B release origin/release"

	encrypted, err := EncryptCommandString(plaintext)
	if err != nil {
		t.Fatalf("EncryptCommandString returned error: %v", err)
	}
	if encrypted == "" {
		t.Fatal("expected ciphertext to be non-empty")
	}
	if strings.Contains(encrypted, plaintext) {
		t.Fatalf("expected ciphertext to hide plaintext, got %q", encrypted)
	}

	decrypted, err := DecryptCommandString(encrypted)
	if err != nil {
		t.Fatalf("DecryptCommandString returned error: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("expected decrypted plaintext %q, got %q", plaintext, decrypted)
	}
}

func TestDecryptCommandStringRejectsInvalidCiphertext(t *testing.T) {
	_, err := DecryptCommandString("not-valid-base64")
	if err == nil {
		t.Fatal("expected DecryptCommandString to fail for invalid ciphertext")
	}
}

func TestDecryptCommandString(t *testing.T) {
	encString := "OQ4dvI5c25ioJA3FqabWQbhaUsUl1Z7oD0JVxD5ReWgeu+bpbANGgucwzAGUGT0bokSDTQsoyxbNbhq3aJEZRxZbjC96yUj7iiTPVc3Cb1f5s67l0Q00kETjy9/DCRhCFnVm621xIbYtfEdnB5JWkpSiOhw+S85GzxQLyM5wv4bFpM1nD8nc4Wki9IaXQGvWrgT8LCapcs5rUTpGSn4wo0fKiJ/RnnoDuC8r7stwwUn4eiLC6SAaymIgz/KysTYC22Ux"
	commandString, err := DecryptCommandString(encString)
	t.Log(commandString)
	if err != nil {
		t.Fatalf("DecryptCommandString returned error: %v", err)
	}
}
