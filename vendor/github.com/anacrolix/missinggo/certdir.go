package missinggo

import (
	"crypto/tls"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func LoadCertificateDir(dir string) (certs []tls.Certificate, err error) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	defer d.Close()
	const defaultPEMFile = "default.pem"
	if p := filepath.Join(dir, defaultPEMFile); FilePathExists(p) {
		cert, err := tls.LoadX509KeyPair(p, p)
		if err == nil {
			certs = append(certs, cert)
		} else {
			log.Printf("error loading default certicate: %s", err)
		}
	}
	files, err := d.Readdir(-1)
	if err != nil {
		return
	}
	for _, f := range files {
		if f.Name() == defaultPEMFile {
			continue
		}
		if !strings.HasSuffix(f.Name(), ".pem") {
			continue
		}
		p := filepath.Join(dir, f.Name())
		cert, err := tls.LoadX509KeyPair(p, p)
		if err != nil {
			log.Printf("error loading key pair from %q: %s", p, err)
			continue
		}
		certs = append(certs, cert)
	}
	return
}
