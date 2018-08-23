package missinggo

import (
	"crypto/tls"
	"strings"
)

// Select the best named certificate per the usual behaviour if
// c.GetCertificate is nil, and c.NameToCertificate is not.
func BestNamedCertificate(c *tls.Config, clientHello *tls.ClientHelloInfo) (*tls.Certificate, bool) {
	name := strings.ToLower(clientHello.ServerName)
	for len(name) > 0 && name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}

	if cert, ok := c.NameToCertificate[name]; ok {
		return cert, true
	}

	// try replacing labels in the name with wildcards until we get a
	// match.
	labels := strings.Split(name, ".")
	for i := range labels {
		labels[i] = "*"
		candidate := strings.Join(labels, ".")
		if cert, ok := c.NameToCertificate[candidate]; ok {
			return cert, true
		}
	}

	return nil, false
}
