package api

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// StartServer starts the Gin API server over TLS (server-auth). If a CA file is
// provided and readable, client certificates are verified when presented
// (optional client auth); if caFile is empty or absent, the server still runs
// with server-only TLS and simply does not request/verify client certs. This
// supports the "HTTPS everywhere, mTLS optional" deployment model.
func StartServer(addr, certFile, keyFile, caFile string, r *gin.Engine) error {
	tlsConfig := &tls.Config{}

	if caFile != "" {
		if caCert, err := os.ReadFile(caFile); err == nil {
			caCertPool := x509.NewCertPool()
			if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
				return errors.New("failed to parse CA certificate")
			}
			tlsConfig.ClientCAs = caCertPool
			tlsConfig.ClientAuth = tls.VerifyClientCertIfGiven
		}
		// If the CA file was named but unreadable, fall through to server-only
		// TLS rather than failing startup -- the CA is only needed to verify a
		// client cert, which is optional in this model.
	}

	server := &http.Server{
		Addr:      addr,
		Handler:   r,
		TLSConfig: tlsConfig,
	}

	return server.ListenAndServeTLS(certFile, keyFile)
}

// StartLocalServer starts the Gin API server without mTLS.
func StartLocalServer(addr string, r *gin.Engine) error {
	server := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	return server.ListenAndServe()
}
