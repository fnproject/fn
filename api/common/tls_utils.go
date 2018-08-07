package common

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// A simple TLS Config generator using cert/key
func NewTLSSimple(certPath, keyPath string) (*tls.Config, error) {

	err := checkFile(certPath)
	if err != nil {
		return nil, err
	}

	err = checkFile(keyPath)
	if err != nil {
		return nil, err
	}

	// Load the certificates from disk
	certificate, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("Could not load server key pair: %s", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{certificate},
	}, nil
}

// Add a Client CA
func AddClientCA(tlsConf *tls.Config, clientCAPath string) error {

	err := checkFile(clientCAPath)
	if err != nil {
		return err
	}
	// Create a certificate pool from the certificate authority
	authority, err := ioutil.ReadFile(clientCAPath)
	if err != nil {
		return fmt.Errorf("Could not read client CA (%s) certificate: %s", clientCAPath, err)
	}

	tlsConf.ClientAuth = tls.RequireAndVerifyClientCert
	if tlsConf.ClientCAs == nil {
		tlsConf.ClientCAs = x509.NewCertPool()
	}

	if ok := tlsConf.ClientCAs.AppendCertsFromPEM(authority); !ok {
		return errors.New("Failed to append client certs")
	}
	return nil
}

// Add CA
func AddCA(tlsConf *tls.Config, caPath string) error {

	err := checkFile(caPath)
	if err != nil {
		return err
	}

	ca, err := ioutil.ReadFile(caPath)
	if err != nil {
		return fmt.Errorf("could not read ca (%s) certificate: %s", caPath, err)
	}

	if tlsConf.RootCAs == nil {
		tlsConf.RootCAs = x509.NewCertPool()
	}

	// Append the certificates from the CA
	if ok := tlsConf.RootCAs.AppendCertsFromPEM(ca); !ok {
		return errors.New("failed to append ca certs")
	}

	return nil
}

func checkFile(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("Unable to resolve %v for TLS: please specify a valid and readable file", path)
	}
	_, err = os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("Cannot stat %v for TLS: please specify a valid and readable file", absPath)
	}
	return nil
}
