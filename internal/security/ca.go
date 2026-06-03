package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"time"
)

// GenerateCA creates a new Certificate Authority.
// It returns the PEM encoded CA certificate and the PEM encoded private key.
func GenerateCA() ([]byte, []byte, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"VLX_FrameFlow"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // 10 years
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}

	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caBytes})

	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})

	return caPEM, privPEM, nil
}

// EnsureLocalCA checks if CA files exist at caCertPath and caKeyPath.
// If they don't, it generates a new CA and saves it to those paths.
func EnsureLocalCA(caCertPath, caKeyPath string) error {
	_, errCert := os.Stat(caCertPath)
	_, errKey := os.Stat(caKeyPath)

	if errCert == nil && errKey == nil {
		// CA already exists
		return nil
	}

	caCert, caKey, err := GenerateCA()
	if err != nil {
		return err
	}

	if err := os.WriteFile(caCertPath, caCert, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(caKeyPath, caKey, 0600); err != nil {
		return err
	}

	return nil
}

// GenerateServerCert generates a server certificate and private key signed by the given CA.
func GenerateServerCert(caCertPEM, caKeyPEM []byte) ([]byte, []byte, error) {
	return generateCert(caCertPEM, caKeyPEM, "VLX_FrameFlow Server", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, []string{"localhost"})
}

// GenerateClientCert generates a client certificate and private key signed by the given CA.
func GenerateClientCert(caCertPEM, caKeyPEM []byte, clientName string) ([]byte, []byte, error) {
	return generateCert(caCertPEM, caKeyPEM, clientName, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, nil)
}

func generateCert(caCertPEM, caKeyPEM []byte, commonName string, extKeyUsage []x509.ExtKeyUsage, dnsNames []string) ([]byte, []byte, error) {
	caCertBlock, _ := pem.Decode(caCertPEM)
	if caCertBlock == nil {
		return nil, nil, errors.New("failed to decode CA cert PEM")
	}
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}

	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock == nil {
		return nil, nil, errors.New("failed to decode CA key PEM")
	}
	var caKey interface{}
	caKey, err = x509.ParseECPrivateKey(caKeyBlock.Bytes)
	if err != nil {
		caKey, err = x509.ParsePKCS8PrivateKey(caKeyBlock.Bytes)
		if err != nil {
			return nil, nil, errors.New("failed to parse CA private key")
		}
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, err
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"VLX_FrameFlow"},
			CommonName:   commonName,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(1, 0, 0), // 1 year validity
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: extKeyUsage,
		DNSNames:    dnsNames,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, caCert, &priv.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})

	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})

	return certPEM, privPEM, nil
}
