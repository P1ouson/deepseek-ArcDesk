package wecom

import (
	"bytes"
	"testing"
)

func TestPKCS7PadUnpad(t *testing.T) {
	plain := []byte("hello")
	padded, err := pkcs7Pad(plain, aesBlockSize())
	if err != nil {
		t.Fatal(err)
	}
	out, err := pkcs7Unpad(padded, aesBlockSize())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, plain) {
		t.Fatalf("got %q want %q", out, plain)
	}
}

func TestNewClientRequiresFields(t *testing.T) {
	if _, err := NewClient("", "secret", "1000002"); err == nil {
		t.Fatal("expected corp id error")
	}
	if _, err := NewClient("corp", "", "1000002"); err == nil {
		t.Fatal("expected secret error")
	}
	if _, err := NewClient("corp", "secret", "bad"); err == nil {
		t.Fatal("expected agent id error")
	}
}

func aesBlockSize() int {
	return 16
}
