package aibot

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"testing"
)

// TestDecryptFileUnpaddedAESKey 真机形态密钥（43 字符无 padding base64）解密 roundtrip。
func TestDecryptFileUnpaddedAESKey(t *testing.T) {
	rawKey := "2WgPY8aqRkGJn1SD3+yrLocnAbT0h0pUpYUW4lFmGo4"
	key, err := base64.RawStdEncoding.DecodeString(rawKey)
	if err != nil {
		t.Fatalf("构造密钥: %v", err)
	}
	got, err := decryptRoundtrip(key, rawKey)
	if err != nil {
		t.Fatalf("无 padding 密钥解密失败: %v", err)
	}
	if got != "hello wecom attachment" {
		t.Fatalf("解密结果不符: %q", got)
	}
}

// TestDecryptFilePaddedAESKey 带 padding 密钥回归（标准 44 字符形态）。
func TestDecryptFilePaddedAESKey(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	got, err := decryptRoundtrip(key, base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatalf("带 padding 密钥解密失败: %v", err)
	}
	if got != "hello wecom attachment" {
		t.Fatalf("解密结果不符: %q", got)
	}
}

// decryptRoundtrip 以 IV=密钥前 16 字节加密样本后经 DecryptFile 解密，返回明文。
func decryptRoundtrip(key []byte, aesKey string) (string, error) {
	plain := []byte("hello wecom attachment")
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	padded := Pkcs7Pad(plain, Pkcs7BlockSize)
	ct := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, key[:aes.BlockSize]).CryptBlocks(ct, padded)
	out, err := DecryptFile(ct, aesKey)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
