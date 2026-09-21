package auth

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"
	"time"
)

func TestHashPassword_Roundtrip(t *testing.T) {
	tests := []struct {
		password string
	}{
		{"correct-password"},
		{"p@ssw0rd!with$pecial#chars"},
		{"x"},
		{"                  "},
		{"こんにちは"},
	}
	for _, tt := range tests {
		t.Run(tt.password, func(t *testing.T) {
			hash, err := HashPassword(tt.password)
			if err != nil {
				t.Fatalf("HashPassword(%q): %v", tt.password, err)
			}
			if hash == "" {
				t.Fatal("expected non-empty hash")
			}
			if len(hash) < 90 {
				t.Fatalf("hash too short: %q", hash)
			}
			valid, err := CheckPassword(hash, tt.password)
			if err != nil {
				t.Fatalf("CheckPassword: %v", err)
			}
			if !valid {
				t.Fatalf("password did not verify")
			}
		})
	}
}

func TestHashPassword_WrongPassword(t *testing.T) {
	hash, err := HashPassword("correct")
	if err != nil {
		t.Fatal(err)
	}
	valid, err := CheckPassword(hash, "wrong")
	if err != nil {
		t.Fatal(err)
	}
	if valid {
		t.Fatal("expected false for wrong password")
	}
}

func TestHashPassword_DifferentSalts(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Fatal("expected different hashes due to random salt")
	}
	v1, _ := CheckPassword(h1, "same")
	v2, _ := CheckPassword(h2, "same")
	if !v1 || !v2 {
		t.Fatal("both hashes should verify")
	}
}

func TestHashPassword_InvalidHash(t *testing.T) {
	tests := []string{
		"",
		"$argon2id$v=19$m=0,t=1,p=4$abc$def",
		"$argon2id$v=19$m=65536,t=1,p=4$short",
		"not-a-hash",
	}
	for _, h := range tests {
		t.Run(h[:min(30, len(h))], func(t *testing.T) {
			_, err := CheckPassword(h, "pw")
			if err == nil {
				t.Fatalf("expected error for invalid hash %q", h)
			}
		})
	}
}

func TestHashPassword_ParseHashComponents(t *testing.T) {
	hash, _ := HashPassword("test")
	if !strings.Contains(hash, "$argon2id$") {
		t.Fatal("missing argon2id prefix")
	}
	if !strings.Contains(hash, "$v=19$") {
		t.Fatal("missing version")
	}
	if !strings.Contains(hash, "m=65536,") {
		t.Fatal("missing memory param")
	}
	if !strings.Contains(hash, "t=3,") {
		t.Fatal("missing time param (t=3)")
	}
	if !strings.Contains(hash, "p=4$") {
		t.Fatal("missing parallelism param")
	}
	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		t.Fatalf("expected 6 parts, got %d: %v", len(parts), parts)
	}
	salt, _ := base64.RawStdEncoding.DecodeString(parts[4])
	if len(salt) != 16 {
		t.Fatalf("expected 16-byte salt, got %d", len(salt))
	}
}

func TestGenerateToken_Valid(t *testing.T) {
	os.Setenv("HIVESTACK_JWT_SECRET", "test-secret-key-12345")
	defer os.Unsetenv("HIVESTACK_JWT_SECRET")

	token, err := GenerateToken("user-1", "tenant-1", []string{"vm.list", "vm.create"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("empty token")
	}

	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != "user-1" {
		t.Errorf("UserID = %q, want user-1", claims.UserID)
	}
	if claims.TenantID != "tenant-1" {
		t.Errorf("TenantID = %q, want tenant-1", claims.TenantID)
	}
	if len(claims.Scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d: %v", len(claims.Scopes), claims.Scopes)
	}
	if claims.Scopes[0] != "vm.list" {
		t.Errorf("scopes[0] = %q", claims.Scopes[0])
	}
}

func TestGenerateToken_EmptySecret(t *testing.T) {
	os.Unsetenv("HIVESTACK_JWT_SECRET")
	_, err := GenerateToken("u", "t", nil, 0)
	if err == nil {
		t.Fatal("expected error for missing secret")
	}
}

func TestGenerateToken_CustomExpiry(t *testing.T) {
	os.Setenv("HIVESTACK_JWT_SECRET", "test")
	defer os.Unsetenv("HIVESTACK_JWT_SECRET")

	short := 5 * time.Second
	token, err := GenerateToken("u", "t", nil, short)
	if err != nil {
		t.Fatal(err)
	}
	claims, _ := ValidateToken(token)
	if claims == nil {
		t.Fatal("nil claims")
	}
	if claims.ExpiresAt == nil {
		t.Fatal("missing expiry")
	}
	defaultToken, _ := GenerateToken("u", "t", nil, 0)
	defaultClaims, _ := ValidateToken(defaultToken)
	if defaultClaims.ExpiresAt.Sub(claims.ExpiresAt.Time) <= 0 {
		t.Fatal("default expiry should be longer than custom short expiry")
	}
}

func TestValidateToken_InvalidSignature(t *testing.T) {
	os.Setenv("HIVESTACK_JWT_SECRET", "secret-a")
	defer os.Unsetenv("HIVESTACK_JWT_SECRET")

	token, _ := GenerateToken("u", "t", nil, 0)

	os.Setenv("HIVESTACK_JWT_SECRET", "secret-b")
	_, err := ValidateToken(token)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestValidateToken_EmptyToken(t *testing.T) {
	os.Setenv("HIVESTACK_JWT_SECRET", "secret")
	defer os.Unsetenv("HIVESTACK_JWT_SECRET")

	_, err := ValidateToken("")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestValidateToken_NotAJWT(t *testing.T) {
	os.Setenv("HIVESTACK_JWT_SECRET", "secret")
	defer os.Unsetenv("HIVESTACK_JWT_SECRET")

	_, err := ValidateToken("not.a.jwt.token")
	if err == nil {
		t.Fatal("expected error for non-JWT")
	}
}

func TestClaimsStruct(t *testing.T) {
	claims := &Claims{UserID: "u1", TenantID: "t1", Scopes: []string{"read"}}
	if claims.UserID != "u1" {
		t.Errorf("UserID = %q", claims.UserID)
	}
}

func TestDefaultExpiry(t *testing.T) {
	if DefaultExpiry != 24*time.Hour {
		t.Errorf("DefaultExpiry = %v, want 24h", DefaultExpiry)
	}
}

func TestGenerateTokenPair(t *testing.T) {
	os.Setenv("HIVESTACK_JWT_SECRET", "test-secret-key-12345")
	defer os.Unsetenv("HIVESTACK_JWT_SECRET")

	accessToken, refreshToken, err := GenerateTokenPair("user-1", "tenant-1", []string{"vm.list", "vm.create"})
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}
	if accessToken == "" {
		t.Fatal("empty access token")
	}
	if refreshToken == "" {
		t.Fatal("empty refresh token")
	}

	// Validate access token
	claims, err := ValidateToken(accessToken)
	if err != nil {
		t.Fatalf("ValidateToken(access): %v", err)
	}
	if claims.UserID != "user-1" {
		t.Errorf("UserID = %q, want user-1", claims.UserID)
	}
	if claims.TenantID != "tenant-1" {
		t.Errorf("TenantID = %q, want tenant-1", claims.TenantID)
	}
	if len(claims.Scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(claims.Scopes))
	}

	// Validate refresh token
	newToken, rtClaims, err := ValidateRefreshToken(refreshToken)
	if err != nil {
		t.Fatalf("ValidateRefreshToken: %v", err)
	}
	if rtClaims.UserID != "user-1" {
		t.Errorf("refresh UserID = %q, want user-1", rtClaims.UserID)
	}
	if rtClaims.TenantID != "tenant-1" {
		t.Errorf("refresh TenantID = %q, want tenant-1", rtClaims.TenantID)
	}
	if newToken == "" {
		t.Fatal("expected new refresh token after rotation")
	}
	if newToken == refreshToken {
		t.Fatal("expected rotated token to differ from original")
	}
}

func TestValidateRefreshToken_InvalidToken(t *testing.T) {
	_, _, err := ValidateRefreshToken("invalid-token")
	if err == nil {
		t.Fatal("expected error for invalid refresh token")
	}
}

func TestValidateRefreshToken_EmptyToken(t *testing.T) {
	_, _, err := ValidateRefreshToken("")
	if err == nil {
		t.Fatal("expected error for empty refresh token")
	}
}

func TestValidateRefreshToken_Rotation(t *testing.T) {
	os.Setenv("HIVESTACK_JWT_SECRET", "test-secret")
	defer os.Unsetenv("HIVESTACK_JWT_SECRET")

	// Generate a token
	_, refreshToken, err := GenerateTokenPair("user-2", "tenant-2", []string{"read"})
	if err != nil {
		t.Fatal(err)
	}

	// First validation should succeed and rotate
	newToken, _, err := ValidateRefreshToken(refreshToken)
	if err != nil {
		t.Fatalf("first validation: %v", err)
	}

	// Second validation with old token should fail (already rotated)
	_, _, err = ValidateRefreshToken(refreshToken)
	if err == nil {
		t.Fatal("expected error for reused (rotated) refresh token")
	}

	// New token should work
	_, _, err = ValidateRefreshToken(newToken)
	if err != nil {
		t.Fatalf("new token validation: %v", err)
	}
}

func TestGenerateRefreshToken_Unique(t *testing.T) {
	t1, err := GenerateRefreshToken("u", "t", nil)
	if err != nil {
		t.Fatal(err)
	}
	t2, err := GenerateRefreshToken("u", "t", nil)
	if err != nil {
		t.Fatal(err)
	}
	if t1 == t2 {
		t.Fatal("expected different refresh tokens")
	}
}

func TestClaimsStruct_WithRefreshToken(t *testing.T) {
	claims := &Claims{
		UserID:       "u1",
		TenantID:     "t1",
		Scopes:       []string{"read"},
		RefreshToken: "some-refresh-token",
	}
	if claims.RefreshToken != "some-refresh-token" {
		t.Errorf("RefreshToken = %q", claims.RefreshToken)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
