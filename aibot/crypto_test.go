package aibot

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"testing"
)

// encryptForTest 用 PKCS#7(32) 填充 + AES-256-CBC 加密，仅供测试构造密文（镜像 DecryptFile 的逆过程）。
func encryptForTest(t testing.TB, plain, key []byte) []byte {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("invalid test key: %v", err)
	}
	padded := Pkcs7Pad(plain, Pkcs7BlockSize)
	dst := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, key[:aes.BlockSize]).CryptBlocks(dst, padded)
	return dst
}

// testAesKey 构造一个固定 32 字节 AES-256 密钥及其 Base64 编码。
func testAesKey(t testing.TB) ([]byte, string) {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	return key, base64.StdEncoding.EncodeToString(key)
}

// ========== PKCS#7 单测 ==========

func TestPkcs7PadUnpadRoundTrip(t *testing.T) {
	cases := [][]byte{
		nil,
		[]byte(""),
		[]byte("a"),
		bytes.Repeat([]byte("a"), 16),
		bytes.Repeat([]byte("a"), 31),
		bytes.Repeat([]byte("a"), 32),
		bytes.Repeat([]byte("a"), 33),
		bytes.Repeat([]byte("a"), 64),
	}
	for _, plain := range cases {
		padded := Pkcs7Pad(plain, Pkcs7BlockSize)
		if len(padded)%Pkcs7BlockSize != 0 {
			t.Errorf("padded length %d not a multiple of %d", len(padded), Pkcs7BlockSize)
			continue
		}
		got, err := Pkcs7Unpad(padded, Pkcs7BlockSize)
		if err != nil {
			t.Errorf("Pkcs7Unpad error: %v", err)
			continue
		}
		if !bytes.Equal(got, plain) {
			t.Errorf("round-trip mismatch: got %q want %q", got, plain)
		}
	}
}

func TestPkcs7UnpadInvalid(t *testing.T) {
	cases := [][]byte{
		{},                         // 空
		{0},                        // padLen=0 非法
		{33},                       // padLen=33 超过 blockSize
		{1, 2},                     // 尾字节=2 但前一字节不匹配
		bytes.Repeat([]byte{5}, 4), // padLen=5 > len=4
	}
	for _, c := range cases {
		if _, err := Pkcs7Unpad(c, Pkcs7BlockSize); err == nil {
			t.Errorf("Pkcs7Unpad(%v) should fail", c)
		}
	}
}

// ========== DecryptFile 往返 ==========

func TestDecryptFileRoundTrip(t *testing.T) {
	key, aesKey := testAesKey(t)
	cases := [][]byte{
		[]byte("hello"),
		[]byte(""),
		bytes.Repeat([]byte("x"), 16),
		bytes.Repeat([]byte("x"), 31),
		bytes.Repeat([]byte("x"), 32),
		bytes.Repeat([]byte("x"), 33),
		bytes.Repeat([]byte("x"), 100),
		bytes.Repeat([]byte{0}, 200),
	}
	for _, plain := range cases {
		encrypted := encryptForTest(t, plain, key)
		got, err := DecryptFile(encrypted, aesKey)
		if err != nil {
			t.Errorf("DecryptFile(plain len=%d) error: %v", len(plain), err)
			continue
		}
		if !bytes.Equal(got, plain) {
			t.Errorf("DecryptFile round-trip mismatch (len=%d): got %d bytes, want %d", len(plain), len(got), len(plain))
		}
	}
}

// ========== 空输入 / 参数校验 ==========

func TestDecryptFileEmptyInput(t *testing.T) {
	_, aesKey := testAesKey(t)
	if _, err := DecryptFile(nil, aesKey); err == nil {
		t.Error("DecryptFile(nil) should fail")
	}
	if _, err := DecryptFile([]byte{}, aesKey); err == nil {
		t.Error("DecryptFile(empty) should fail")
	}
	key, _ := testAesKey(t)
	encrypted := encryptForTest(t, []byte("data"), key)
	if _, err := DecryptFile(encrypted, ""); err == nil {
		t.Error("DecryptFile with empty aesKey should fail")
	}
	// 非法 base64
	if _, err := DecryptFile(encrypted, "not!!!base64"); err == nil {
		t.Error("DecryptFile with invalid base64 aesKey should fail")
	}
	// 长度非法的 key（解码后非 16/24/32 字节）
	badLenKey := base64.StdEncoding.EncodeToString([]byte("short")) // 5 字节
	if _, err := DecryptFile(encrypted, badLenKey); err == nil {
		t.Error("DecryptFile with invalid-length key should fail")
	}
	// 密文长度非块大小倍数
	if _, err := DecryptFile([]byte{1, 2, 3}, aesKey); err == nil {
		t.Error("DecryptFile with non-block-aligned ciphertext should fail")
	}
}

// ========== 错误 key ==========

func TestDecryptFileWrongKey(t *testing.T) {
	key1, _ := testAesKey(t)
	key2 := make([]byte, 32)
	for i := range key2 {
		key2[i] = byte(i + 100) // 不同的有效长度 key
	}
	wrongAesKey := base64.StdEncoding.EncodeToString(key2)

	plain := bytes.Repeat([]byte("sensitive payload "), 8) // 较大数据降低巧合概率
	encrypted := encryptForTest(t, plain, key1)

	got, err := DecryptFile(encrypted, wrongAesKey)
	// 错误 key：要么解密失败（padding 非法，常见），要么解出乱码但绝不等于原文
	if err == nil && bytes.Equal(got, plain) {
		t.Error("wrong key must not recover the original plaintext")
	}
}

// ========== Fuzz 往返 ==========

func FuzzDecryptFile(f *testing.F) {
	key, aesKey := testAesKey(f)
	f.Add([]byte("hello world"))
	f.Add(bytes.Repeat([]byte("a"), 31))
	f.Add(bytes.Repeat([]byte("a"), 32))
	f.Add(make([]byte, 0))
	f.Fuzz(func(t *testing.T, plain []byte) {
		if len(plain) > 8192 {
			return
		}
		encrypted := encryptForTest(t, plain, key)
		got, err := DecryptFile(encrypted, aesKey)
		if err != nil {
			t.Fatalf("DecryptFile error: %v", err)
		}
		if !bytes.Equal(got, plain) {
			t.Fatalf("round-trip mismatch: got %d bytes, want %d", len(got), len(plain))
		}
	})
}
