// Package vault 提供 site.yaml 敏感字段的 AES 加密（方案阶段三：!vault 标记）。
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
)

const prefix = "!vault:"

// DeriveKey 从口令派生 32 字节 AES 密钥。
func DeriveKey(passphrase string) []byte {
	sum := sha256.Sum256([]byte(passphrase))
	return sum[:]
}

// Encrypt 加密明文，返回 !vault:<base64> 形式。
func Encrypt(plaintext, passphrase string) (string, error) {
	if passphrase == "" {
		return "", fmt.Errorf("加密口令不能为空")
	}
	block, err := aes.NewCipher(DeriveKey(passphrase))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return prefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt 解密 !vault: 标记的密文；非 vault 字符串原样返回。
func Decrypt(value, passphrase string) (string, error) {
	if !strings.HasPrefix(value, prefix) {
		return value, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return "", fmt.Errorf("vault 密文损坏: %w", err)
	}
	block, err := aes.NewCipher(DeriveKey(passphrase))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("vault 密文过短")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败（口令可能错误）: %w", err)
	}
	return string(plain), nil
}

// IsVault 判断是否为加密标记值。
func IsVault(value string) bool {
	return strings.HasPrefix(value, prefix)
}

// PassphraseFromEnv 读取 WPGCTL_VAULT_KEY，缺失时返回错误。
func PassphraseFromEnv() (string, error) {
	p := os.Getenv("WPGCTL_VAULT_KEY")
	if p == "" {
		return "", fmt.Errorf("请设置环境变量 WPGCTL_VAULT_KEY")
	}
	return p, nil
}
