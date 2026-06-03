package api

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// StartServer configures mTLS and starts the Gin API server.
func StartServer(addr, certFile, keyFile, caFile string, r *gin.Engine) error {
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return err
	}

	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
		return errors.New("failed to parse CA certificate")
	}

	tlsConfig := &tls.Config{
		ClientCAs:  caCertPool,
		ClientAuth: tls.VerifyClientCertIfGiven,
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
