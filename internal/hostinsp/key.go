package hostinsp

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	gossh "golang.org/x/crypto/ssh"
)

func readFileOrEmpty(path string) ([]byte, error) {
	b, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return nil, err
	}
	return b, nil
}

// makeKeySigner 解析 PEM 私钥（支持加密私钥），返回签名器。
func makeKeySigner(pemBytes []byte, passphrase string) (gossh.Signer, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("私钥 PEM 格式无效")
	}
	switch block.Type {
	case "OPENSSH PRIVATE KEY":
		// OpenSSH 新格式（口令可选）
		var signer gossh.Signer
		var err error
		if passphrase != "" {
			signer, err = gossh.ParsePrivateKeyWithPassphrase(pemBytes, []byte(passphrase))
		} else {
			signer, err = gossh.ParsePrivateKey(pemBytes)
		}
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "passphrase") || strings.Contains(err.Error(), "encrypt") {
				return nil, fmt.Errorf("私钥已加密，请填写私钥口令")
			}
			return nil, err
		}
		return signer, nil
	case "PRIVATE KEY":
		// PKCS8（可能加密）
		if !strings.Contains(string(block.Headers["Proc-Type"]), "ENCRYPTED") {
			if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
				return gossh.NewSignerFromKey(key)
			}
		}
		signer, err := gossh.ParsePrivateKeyWithPassphrase(pemBytes, []byte(passphrase))
		if err != nil {
			return nil, fmt.Errorf("私钥解析失败（可能口令错误）: %w", err)
		}
		return signer, nil
	default:
		// 传统 PEM（RSA/EC/DSA，可能按 Proc-Type: 4,ENCRYPTED 加密）
		if isEncryptedPEM(block) {
			dec, err := x509.DecryptPEMBlock(block, []byte(passphrase))
			if err != nil {
				return nil, fmt.Errorf("私钥解密失败（可能口令错误）: %w", err)
			}
			var key any
			if block.Type == "RSA PRIVATE KEY" {
				key, err = x509.ParsePKCS1PrivateKey(dec)
			} else if block.Type == "EC PRIVATE KEY" {
				key, err = x509.ParseECPrivateKey(dec)
			} else {
				key, err = x509.ParsePKCS8PrivateKey(dec)
			}
			if err != nil {
				return nil, err
			}
			return gossh.NewSignerFromKey(key)
		}
		signer, err := gossh.ParsePrivateKey(pemBytes)
		if err != nil {
			return nil, err
		}
		return signer, nil
	}
}

func isEncryptedPEM(block *pem.Block) bool {
	return len(block.Headers) > 0 && strings.Contains(block.Headers["Proc-Type"], "ENCRYPTED")
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
