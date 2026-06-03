package security

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateCA(t *testing.T) {
	caCertPEM, caKeyPEM, err := GenerateCA()
	assert.NoError(t, err)
	assert.NotEmpty(t, caCertPEM)
	assert.NotEmpty(t, caKeyPEM)

	block, _ := pem.Decode(caCertPEM)
	assert.NotNil(t, block)
	cert, err := x509.ParseCertificate(block.Bytes)
	assert.NoError(t, err)
	assert.True(t, cert.IsCA)
}

func TestEnsureLocalCA(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "ca.crt")
	keyPath := filepath.Join(tmpDir, "ca.key")

	err := EnsureLocalCA(certPath, keyPath)
	assert.NoError(t, err)

	_, err = os.Stat(certPath)
	assert.NoError(t, err)
	_, err = os.Stat(keyPath)
	assert.NoError(t, err)

	// Call again to test the path where files already exist
	err = EnsureLocalCA(certPath, keyPath)
	assert.NoError(t, err)
}

func TestGenerateServerCert(t *testing.T) {
	caCertPEM, caKeyPEM, err := GenerateCA()
	assert.NoError(t, err)

	srvCertPEM, srvKeyPEM, err := GenerateServerCert(caCertPEM, caKeyPEM)
	assert.NoError(t, err)
	assert.NotEmpty(t, srvCertPEM)
	assert.NotEmpty(t, srvKeyPEM)

	block, _ := pem.Decode(srvCertPEM)
	assert.NotNil(t, block)
	cert, err := x509.ParseCertificate(block.Bytes)
	assert.NoError(t, err)
	assert.Equal(t, "VLX_FrameFlow Server", cert.Subject.CommonName)
}

func TestGenerateClientCert(t *testing.T) {
	caCertPEM, caKeyPEM, err := GenerateCA()
	assert.NoError(t, err)

	clientName := "TestClient123"
	clientCertPEM, clientKeyPEM, err := GenerateClientCert(caCertPEM, caKeyPEM, clientName)
	assert.NoError(t, err)
	assert.NotEmpty(t, clientCertPEM)
	assert.NotEmpty(t, clientKeyPEM)

	block, _ := pem.Decode(clientCertPEM)
	assert.NotNil(t, block)
	cert, err := x509.ParseCertificate(block.Bytes)
	assert.NoError(t, err)
	assert.Equal(t, clientName, cert.Subject.CommonName)
}
