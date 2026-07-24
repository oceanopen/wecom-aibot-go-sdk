package aibot

import (
	"encoding/base64"
	"regexp"
	"strings"
	"testing"
)

// testEncodingAesKey 构造 32 字节 AES 密钥及其企微 43 位 EncodingAesKey 形式（去掉 base64 填充）。
func testEncodingAesKey(t testing.TB) ([]byte, string) {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	full := base64.StdEncoding.EncodeToString(key) // 32 字节 → 44 字符，单 '=' 结尾
	return key, strings.TrimRight(full, "=")       // 去填充 → 43 字符（企微 key 形态）
}

var sha1HexRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// ========== DecodeEncodingAesKey ==========

func TestDecodeEncodingAesKey(t *testing.T) {
	key, encodingAesKey := testEncodingAesKey(t)

	got, err := DecodeEncodingAesKey(encodingAesKey)
	if err != nil {
		t.Fatalf("DecodeEncodingAesKey error: %v", err)
	}
	if len(got) != 32 {
		t.Errorf("decoded length = %d, want 32", len(got))
	}
	if string(got) != string(key) {
		t.Errorf("decoded key mismatch")
	}

	// 已带 '=' 的完整 base64 形式同样可解码
	full := base64.StdEncoding.EncodeToString(key)
	if _, err := DecodeEncodingAesKey(full); err != nil {
		t.Errorf("DecodeEncodingAesKey(padded form) error: %v", err)
	}

	// 空字符串报错
	if _, err := DecodeEncodingAesKey(""); err == nil {
		t.Error("DecodeEncodingAesKey(\"\") should fail")
	}
	if _, err := DecodeEncodingAesKey("   "); err == nil {
		t.Error("DecodeEncodingAesKey(blank) should fail")
	}

	// 长度非法（解码后非 32 字节）
	short := base64.StdEncoding.EncodeToString([]byte("short")) // 5 字节 → 补 '=' 后仍 5 字节
	if _, err := DecodeEncodingAesKey(short); err == nil {
		t.Error("DecodeEncodingAesKey with invalid length should fail")
	}
}

func TestNewWecomCryptoErrors(t *testing.T) {
	_, encodingAesKey := testEncodingAesKey(t)
	// 空 token
	if _, err := NewWecomCrypto("", encodingAesKey, ""); err == nil {
		t.Error("NewWecomCrypto with empty token should fail")
	}
	// 非法 encodingAesKey
	if _, err := NewWecomCrypto("token", "not-a-valid-key", ""); err == nil {
		t.Error("NewWecomCrypto with invalid encodingAesKey should fail")
	}
	// 合法构造
	wc, err := NewWecomCrypto("token", encodingAesKey, "corp1")
	if err != nil {
		t.Fatalf("NewWecomCrypto error: %v", err)
	}
	if wc == nil {
		t.Fatal("NewWecomCrypto returned nil")
	}
}

// ========== ComputeSignature / VerifySignature ==========

func TestWecomCryptoSignature(t *testing.T) {
	_, encodingAesKey := testEncodingAesKey(t)
	wc, _ := NewWecomCrypto("token123", encodingAesKey, "corp1")

	sig := wc.ComputeSignature("1609459200", "nonceabc", "encryptdata")
	if !sha1HexRe.MatchString(sig) {
		t.Errorf("signature %q is not 40-char lowercase hex", sig)
	}

	// 正确签名校验通过
	if !wc.VerifySignature(sig, "1609459200", "nonceabc", "encryptdata") {
		t.Error("VerifySignature should return true for matching signature")
	}

	// 任一参数变动则校验失败
	if wc.VerifySignature(sig, "1609459201", "nonceabc", "encryptdata") {
		t.Error("VerifySignature should fail when timestamp changed")
	}
	if wc.VerifySignature("0123456789abcdef0123456789abcdef01234567", "1609459200", "nonceabc", "encryptdata") {
		t.Error("VerifySignature should fail for wrong signature")
	}

	// 签名确定性：相同输入产出相同签名
	if wc.ComputeSignature("1609459200", "nonceabc", "encryptdata") != sig {
		t.Error("ComputeSignature should be deterministic")
	}
}

// ========== Encrypt / Decrypt 往返 ==========

func TestWecomCryptoEncryptDecryptRoundTrip(t *testing.T) {
	_, encodingAesKey := testEncodingAesKey(t)
	wc, _ := NewWecomCrypto("token123", encodingAesKey, "corp1")

	cases := []string{
		"hello",
		"",
		"中文消息内容",
		strings.Repeat("a", 16),
		strings.Repeat("a", 31),
		strings.Repeat("a", 32),
		strings.Repeat("a", 100),
	}
	for _, plain := range cases {
		encrypt, sig, err := wc.Encrypt(plain, "1609459200", "nonceabc")
		if err != nil {
			t.Errorf("Encrypt(%q) error: %v", plain, err)
			continue
		}
		if encrypt == "" {
			t.Errorf("Encrypt(%q) returned empty ciphertext", plain)
		}
		if !sha1HexRe.MatchString(sig) {
			t.Errorf("Encrypt(%q) signature not 40-char hex: %s", plain, sig)
		}
		// 签名与密文一致
		if !wc.VerifySignature(sig, "1609459200", "nonceabc", encrypt) {
			t.Errorf("Encrypt(%q) signature mismatch", plain)
		}
		// 解密还原原文
		got, err := wc.Decrypt(encrypt)
		if err != nil {
			t.Errorf("Decrypt(%q) error: %v", plain, err)
			continue
		}
		if got != plain {
			t.Errorf("round-trip mismatch: got %q, want %q", got, plain)
		}
	}
}

// ========== receiveId 校验 ==========

func TestWecomCryptoReceiveIdCheck(t *testing.T) {
	_, encodingAesKey := testEncodingAesKey(t)

	// 加密方 receiveId = corpA
	wcA, _ := NewWecomCrypto("token", encodingAesKey, "corpA")
	encrypt, _, err := wcA.Encrypt("secret msg", "1609459200", "nonce")
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}

	// 同 receiveId 解密成功
	if msg, err := wcA.Decrypt(encrypt); err != nil || msg != "secret msg" {
		t.Errorf("decrypt with matching receiveId: msg=%q err=%v", msg, err)
	}

	// 不同 receiveId 解密失败（尾部 corpA != corpB）
	wcB, _ := NewWecomCrypto("token", encodingAesKey, "corpB")
	if _, err := wcB.Decrypt(encrypt); err == nil {
		t.Error("decrypt with mismatched receiveId should fail")
	}

	// receiveId 为空时不校验尾部，解密成功
	wcEmpty, _ := NewWecomCrypto("token", encodingAesKey, "")
	if msg, err := wcEmpty.Decrypt(encrypt); err != nil || msg != "secret msg" {
		t.Errorf("decrypt with empty receiveId: msg=%q err=%v", msg, err)
	}
}

// ========== Decrypt 错误输入 ==========

func TestWecomCryptoDecryptInvalid(t *testing.T) {
	_, encodingAesKey := testEncodingAesKey(t)
	wc, _ := NewWecomCrypto("token", encodingAesKey, "")

	// 非法 base64
	if _, err := wc.Decrypt("!!!not-base64!!!"); err == nil {
		t.Error("Decrypt with invalid base64 should fail")
	}
	// 长度非块对齐（合法 base64 但字节数非 16 倍数）
	shortCipher := base64.StdEncoding.EncodeToString([]byte{1, 2, 3})
	if _, err := wc.Decrypt(shortCipher); err == nil {
		t.Error("Decrypt with non-block-aligned ciphertext should fail")
	}
}

// ========== Fuzz 往返 ==========

func FuzzWecomCryptoRoundTrip(f *testing.F) {
	_, encodingAesKey := testEncodingAesKey(f)
	wc, err := NewWecomCrypto("token", encodingAesKey, "corp1")
	if err != nil {
		f.Fatalf("NewWecomCrypto error: %v", err)
	}

	f.Add("hello world")
	f.Add("中文 fuzz 测试")
	f.Add("")
	f.Add(strings.Repeat("x", 64))
	f.Fuzz(func(t *testing.T, plain string) {
		encrypt, sig, err := wc.Encrypt(plain, "1609459200", "nonce")
		if err != nil {
			t.Fatalf("Encrypt error: %v", err)
		}
		got, err := wc.Decrypt(encrypt)
		if err != nil {
			t.Fatalf("Decrypt error: %v", err)
		}
		if got != plain {
			t.Fatalf("round-trip mismatch: got %q, want %q", got, plain)
		}
		if !wc.VerifySignature(sig, "1609459200", "nonce", encrypt) {
			t.Fatalf("signature verification failed")
		}
	})
}
