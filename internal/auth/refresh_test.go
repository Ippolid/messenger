package auth

import "testing"

func TestGenerateRefreshToken_Unique(t *testing.T) {
	a, err := generateRefreshToken()
	if err != nil {
		t.Fatalf("generateRefreshToken: %v", err)
	}
	b, err := generateRefreshToken()
	if err != nil {
		t.Fatalf("generateRefreshToken: %v", err)
	}
	if a == b {
		t.Fatal("two generated tokens are equal — not random")
	}
	if len(a) != 64 { // 32 байта в hex
		t.Fatalf("token length = %d, want 64 hex chars", len(a))
	}
}

func TestHashRefreshToken_Deterministic(t *testing.T) {
	const token = "some-refresh-token"
	h1 := hashRefreshToken(token)
	h2 := hashRefreshToken(token)
	if h1 != h2 {
		t.Fatal("hash is not deterministic")
	}
	if h1 == token {
		t.Fatal("hash equals plaintext token")
	}
}
