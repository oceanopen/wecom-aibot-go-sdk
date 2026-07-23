package aibot

// crypto.go 对应 Node src/crypto.ts：DecryptFile 文件附件 AES-256-CBC 解密（PKCS#7 块 32）。

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"fmt"
)

// Pkcs7BlockSize 企微特殊约定的 PKCS#7 填充块大小（32 字节，非标准 AES 的 16 字节）。
const Pkcs7BlockSize = 32

// DecryptFile 使用 AES-256-CBC 解密文件附件，对应 Node decryptFile。
//
// aesKey 为 Base64 编码的 AES-256 密钥；IV 取密钥解码后的前 16 字节；
// 关闭自动 padding（因 PKCS#7 块大小为 32，非默认 16），解密后手动去除 PKCS#7 填充。
func DecryptFile(encrypted []byte, aesKey string) ([]byte, error) {
	// 参数验证
	if len(encrypted) == 0 {
		return nil, errors.New("decryptFile: encrypted is empty or not provided")
	}
	if aesKey == "" {
		return nil, errors.New("decryptFile: aesKey must be a non-empty string")
	}

	// 将 Base64 编码的 aesKey 解码
	key, err := base64.StdEncoding.DecodeString(aesKey)
	if err != nil {
		return nil, fmt.Errorf("decryptFile: invalid base64 aesKey - %s", err.Error())
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("decryptFile: invalid aesKey - %s", err.Error())
	}

	// 加密数据长度须为 AES 块大小（16）的整数倍
	if len(encrypted)%block.BlockSize() != 0 {
		return nil, fmt.Errorf("decryptFile: encrypted length (%d) is not a multiple of block size (%d)", len(encrypted), block.BlockSize())
	}

	// IV 取密钥解码后的前 16 字节
	iv := key[:aes.BlockSize]
	dst := make([]byte, len(encrypted))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(dst, encrypted)

	// 手动去除 PKCS#7 填充（支持 32 字节 block）
	unpadded, err := Pkcs7Unpad(dst, Pkcs7BlockSize)
	if err != nil {
		return nil, fmt.Errorf("decryptFile: Decryption failed - %s. This may indicate corrupted data or an incorrect aesKey.", err.Error())
	}
	return unpadded, nil
}

// Pkcs7Pad 对 data 进行 PKCS#7 填充至 blockSize 的倍数（填充字节值 = 填充长度）。
//
// 注意：当 data 长度恰为 blockSize 倍数时，仍追加一整块填充（标准 PKCS#7 行为）。
func Pkcs7Pad(data []byte, blockSize int) []byte {
	padLen := blockSize - len(data)%blockSize
	padded := make([]byte, len(data)+padLen)
	copy(padded, data)
	for i := len(data); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}
	return padded
}

// Pkcs7Unpad 去除 PKCS#7 填充（blockSize 字节块），校验填充值合法且字节一致。
func Pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("Pkcs7Unpad: empty data")
	}
	padLen := int(data[len(data)-1])
	if padLen < 1 || padLen > blockSize || padLen > len(data) {
		return nil, fmt.Errorf("Pkcs7Unpad: invalid padding value: %d", padLen)
	}
	// 验证所有 padding 字节是否一致
	for i := len(data) - padLen; i < len(data); i++ {
		if data[i] != byte(padLen) {
			return nil, errors.New("Pkcs7Unpad: padding bytes mismatch")
		}
	}
	return data[:len(data)-padLen], nil
}
