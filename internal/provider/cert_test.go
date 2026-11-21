package provider

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dbping/internal/model"
)

// genTestCert 生成临时自签证书对（测试用）。
func genTestCert(t *testing.T, dir string) (cert, key string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tpl := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "dbping-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tpl, &tpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}
	cert = filepath.Join(dir, "client.crt")
	key = filepath.Join(dir, "client.key")
	cf, _ := os.Create(cert)
	_ = pem.Encode(cf, &pem.Block{Type: "CERTIFICATE", Bytes: der})
	cf.Close()
	kf, _ := os.Create(key)
	_ = pem.Encode(kf, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	kf.Close()
	return cert, key
}

// TestMySQLTLSRegistration 验证证书约定参数触发 TLS 注册并写入 DSN，且约定参数不泄漏到 DSN。
func TestMySQLTLSRegistration(t *testing.T) {
	dir := t.TempDir()
	cert, key := genTestCert(t, dir)

	c := &model.Conn{
		Type: "mysql", Host: "127.0.0.1", Port: 3306, User: "u", Password: "p",
		Params: "sslCert=" + cert + "&sslKey=" + key + "&sslSkipVerify=true&custom=1",
	}
	dsn := MySQLDSN(c)
	if !strings.Contains(dsn, "tls=dbping-") {
		t.Fatalf("DSN 应包含注册的 TLS 配置名: %s", dsn)
	}
	if strings.Contains(dsn, "sslCert=") || strings.Contains(dsn, "sslSkipVerify") {
		t.Fatalf("约定参数不应出现在 DSN 中: %s", dsn)
	}
	if !strings.Contains(dsn, "custom=1") {
		t.Fatalf("普通附加参数应保留: %s", dsn)
	}
	// 再次构建（同名注册应幂等，不 panic）
	_ = MySQLDSN(c)
}

// TestMySQLTLSNoCert 无证书参数时不注册、不携带 tls=。
func TestMySQLTLSNoCert(t *testing.T) {
	c := &model.Conn{Type: "mysql", Host: "127.0.0.1", Port: 3306, Params: "custom=1"}
	dsn := MySQLDSN(c)
	if strings.Contains(dsn, "tls=") {
		t.Fatalf("无证书参数不应出现 tls=: %s", dsn)
	}
}

// TestPGCertPassthrough PG 系证书参数透传。
func TestPGCertPassthrough(t *testing.T) {
	c := &model.Conn{
		Type: "pg", Host: "h", Port: 5432, User: "u", Password: "p", Database: "d",
		Params: "sslmode=verify-full&sslrootcert=ca.pem&sslcert=client.crt&sslkey=client.key&sslpassword=x",
	}
	dsn := PGDSN(c, "pgx")
	for _, want := range []string{"sslmode=verify-full", "sslrootcert=ca.pem", "sslcert=client.crt", "sslkey=client.key", "sslpassword=x"} {
		if !strings.Contains(dsn, want) {
			t.Fatalf("DSN 缺少 %s: %s", want, dsn)
		}
	}
}

// TestDmCertPassthrough 达梦证书参数透传。
func TestDmCertPassthrough(t *testing.T) {
	c := &model.Conn{
		Type: "dm", Host: "h", Port: 5236, User: "u", Password: "p",
		Params: "sslEncrypt=4&sslCertPath=cc.pem&sslKeyPath=kk.pem",
	}
	dsn := DmDSN(c)
	for _, want := range []string{"sslEncrypt=4", "sslCertPath=cc.pem", "sslKeyPath=kk.pem"} {
		if !strings.Contains(dsn, want) {
			t.Fatalf("DSN 缺少 %s: %s", want, dsn)
		}
	}
}

// TestMSSQLEncryptPassthrough SQL Server 加密参数透传。
func TestMSSQLEncryptPassthrough(t *testing.T) {
	c := &model.Conn{
		Type: "mssql", Host: "h", Port: 1433, User: "u", Password: "p",
		Params: "encrypt=strict",
	}
	dsn := MSSQLDSN(c)
	if !strings.Contains(dsn, "encrypt=strict") {
		t.Fatalf("DSN 缺少 encrypt=strict: %s", dsn)
	}
}
