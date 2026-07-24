package aibot

// wecom-crypto.go 对应 Node src/wecom-crypto/index.ts：WecomCrypto 回调加解密
//（EncodingAesKey 解码 / SHA1 签名 / AES-256-CBC，明文结构 [16随机][4大端len][msg][receiveId]）。
//
// 独立于 Webhook、WebSocket、Agent 的具体协议形态，统一提供 AES-256-CBC 加解密与 SHA1 签名能力。

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// aesKeyLength AES Key 字节长度，对应 Node CRYPTO_CONSTANTS.AES_KEY_LENGTH。
const aesKeyLength = 32

// DecodeEncodingAesKey 解码企业微信提供的 Base64 EncodingAesKey，对应 Node decodeEncodingAESKey。
//
// 去除首尾空白；若不以 '=' 结尾则补一个 '='（企微 43 位 key 补齐为合法 base64），
// base64 解码后须为 32 字节，否则报错。
func DecodeEncodingAesKey(encodingAesKey string) ([]byte, error) {
	trimmed := strings.TrimSpace(encodingAesKey)
	if trimmed == "" {
		return nil, errors.New("encodingAESKey missing")
	}
	withPadding := trimmed
	if !strings.HasSuffix(trimmed, "=") {
		withPadding = fmt.Sprintf("%s=", trimmed)
	}
	key, err := base64.StdEncoding.DecodeString(withPadding)
	if err != nil {
		return nil, fmt.Errorf("invalid encodingAESKey (base64 decode failed): %w", err)
	}
	if len(key) != aesKeyLength {
		return nil, fmt.Errorf("invalid encodingAESKey (expected %d bytes, got %d)", aesKeyLength, len(key))
	}
	return key, nil
}

// sha1Hex 计算 input 的 SHA1 十六进制摘要，对应 Node sha1Hex。
func sha1Hex(input string) string {
	sum := sha1.Sum([]byte(input))
	return hex.EncodeToString(sum[:])
}

// WecomCrypto 企业微信加解密通用核心，对应 Node WecomCrypto。
//
// 持有 token（签名）、aesKey/iv（AES-256-CBC）与 receiveId（corpId/botId，解密校验、加密追加）。
type WecomCrypto struct {
	token     string // 签名校验用 token
	aesKey    []byte // 解码后的 AES-256 密钥
	iv        []byte // IV（aesKey 前 16 字节）
	receiveId string // corpId 或 botId（解密时校验尾部、加密时追加）
}

// NewWecomCrypto 构造 WecomCrypto，对应 Node new WecomCrypto(token, encodingAESKey, receiveId?)。
//
// token 为空报错；encodingAesKey 经 DecodeEncodingAesKey 解码为 32 字节密钥。
// receiveId 为空表示不校验/不追加（对应 Node receiveId 缺省）。
func NewWecomCrypto(token, encodingAesKey, receiveId string) (*WecomCrypto, error) {
	if token == "" {
		return nil, errors.New("token is required")
	}
	aesKey, err := DecodeEncodingAesKey(encodingAesKey)
	if err != nil {
		return nil, err
	}
	return &WecomCrypto{
		token:     token,
		aesKey:    aesKey,
		iv:        aesKey[:16],
		receiveId: receiveId,
	}, nil
}

// ComputeSignature 计算企业微信消息签名，对应 Node computeSignature。
//
// 将 [token, timestamp, nonce, encrypt] 字典序排序后拼接，取 SHA1 十六进制。
func (c *WecomCrypto) ComputeSignature(timestamp, nonce, encrypt string) string {
	parts := []string{c.token, timestamp, nonce, encrypt}
	sort.Strings(parts)
	return sha1Hex(strings.Join(parts, ""))
}

// VerifySignature 验证企业微信消息签名，对应 Node verifySignature。
//
// 计算 [token,timestamp,nonce,encrypt] 期望签名并与传入签名比对（Node 为直接 ===）。
func (c *WecomCrypto) VerifySignature(signature, timestamp, nonce, encrypt string) bool {
	expected := c.ComputeSignature(timestamp, nonce, encrypt)
	return expected == signature
}

// Decrypt 解密消息，返回纯文本字符串（XML 或 JSON 由上层业务决定），对应 Node decrypt。
//
// 流程：base64 解码 → AES-256-CBC 解密（IV=aesKey[:16]，无自动 padding）→
// PKCS#7(32) 去填充 → 校验长度 → 按 [16随机][4大端len][msg][receiveId] 切分 → 校验 receiveId。
func (c *WecomCrypto) Decrypt(encryptText string) (string, error) {
	cipherText, err := base64.StdEncoding.DecodeString(encryptText)
	if err != nil {
		return "", fmt.Errorf("invalid encrypt (base64 decode failed): %w", err)
	}

	block, err := aes.NewCipher(c.aesKey)
	if err != nil {
		return "", fmt.Errorf("invalid aesKey: %w", err)
	}

	// 密文须为 AES 块大小（16）的整数倍，否则 CBC 解密会 panic（Node 会抛错）
	if len(cipherText)%block.BlockSize() != 0 {
		return "", fmt.Errorf("invalid encrypt length (%d is not a multiple of block size %d)", len(cipherText), block.BlockSize())
	}

	decryptedPadded := make([]byte, len(cipherText))
	cipher.NewCBCDecrypter(block, c.iv).CryptBlocks(decryptedPadded, cipherText)

	decrypted, err := Pkcs7Unpad(decryptedPadded, Pkcs7BlockSize)
	if err != nil {
		return "", err
	}

	if len(decrypted) < 20 {
		return "", fmt.Errorf("invalid payload (expected >=20 bytes, got %d)", len(decrypted))
	}

	// 16 bytes 随机 + 4 bytes 大端 msgLen + msg + receiveId
	msgLen := binary.BigEndian.Uint32(decrypted[16:20])
	const msgStart = 20
	msgEnd := msgStart + int(msgLen)
	if msgEnd > len(decrypted) {
		return "", fmt.Errorf("invalid msg length (msgEnd=%d, total=%d)", msgEnd, len(decrypted))
	}
	msg := string(decrypted[msgStart:msgEnd])

	if c.receiveId != "" {
		trailing := string(decrypted[msgEnd:])
		if trailing != c.receiveId {
			return "", fmt.Errorf("receiveId mismatch (expected \"%s\", got \"%s\")", c.receiveId, trailing)
		}
	}

	return msg, nil
}

// Encrypt 加密明文，返回 base64 密文与对应签名，对应 Node encrypt。
//
// 明文结构：[16随机][4大端len][msg][receiveId]，PKCS#7(32) 填充后 AES-256-CBC 加密。
// 签名按当前 token/timestamp/nonce/encrypt 计算。
func (c *WecomCrypto) Encrypt(plainText, timestamp, nonce string) (encrypt string, signature string, err error) {
	random16 := make([]byte, 16)
	if _, err = rand.Read(random16); err != nil {
		return "", "", fmt.Errorf("generate random bytes failed: %w", err)
	}

	msgBuf := []byte(plainText)
	msgLenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(msgLenBuf, uint32(len(msgBuf)))
	receiveIdBuf := []byte(c.receiveId)

	raw := make([]byte, 0, 16+4+len(msgBuf)+len(receiveIdBuf))
	raw = append(raw, random16...)
	raw = append(raw, msgLenBuf...)
	raw = append(raw, msgBuf...)
	raw = append(raw, receiveIdBuf...)

	padded := Pkcs7Pad(raw, Pkcs7BlockSize)

	block, err := aes.NewCipher(c.aesKey)
	if err != nil {
		return "", "", fmt.Errorf("invalid aesKey: %w", err)
	}
	encrypted := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, c.iv).CryptBlocks(encrypted, padded)
	encrypt = base64.StdEncoding.EncodeToString(encrypted)

	signature = c.ComputeSignature(timestamp, nonce, encrypt)
	return encrypt, signature, nil
}
