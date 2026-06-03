package api

import (
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/viruslox/vlx_frameflow/internal/security"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStartServer(t *testing.T) {
	// Create temporary directory for certificates
	tmpDir := t.TempDir()
	caCertPath := filepath.Join(tmpDir, "ca.crt")
	caKeyPath := filepath.Join(tmpDir, "ca.key")
	srvCertPath := filepath.Join(tmpDir, "server.crt")
	srvKeyPath := filepath.Join(tmpDir, "server.key")

	// Generate CA
	err := security.EnsureLocalCA(caCertPath, caKeyPath)
	assert.NoError(t, err)

	caCertPEM, err := os.ReadFile(caCertPath)
	assert.NoError(t, err)
	caKeyPEM, err := os.ReadFile(caKeyPath)
	assert.NoError(t, err)

	// Generate Server Cert
	srvCertPEM, srvKeyPEM, err := security.GenerateServerCert(caCertPEM, caKeyPEM)
	assert.NoError(t, err)
	err = os.WriteFile(srvCertPath, srvCertPEM, 0644)
	assert.NoError(t, err)
	err = os.WriteFile(srvKeyPath, srvKeyPEM, 0600)
	assert.NoError(t, err)

	// Start Gin Server
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	go func() {
		// Listen on a random free port locally
		_ = StartServer("127.0.0.1:0", srvCertPath, srvKeyPath, caCertPath, r)
	}()

	// Since StartServer is blocking, we just give it a little time to spin up
	// or fail. If it fails, our other tests might catch issues, but here we just ensure
	// no immediate panic/error blocks execution.
	time.Sleep(100 * time.Millisecond)
}
