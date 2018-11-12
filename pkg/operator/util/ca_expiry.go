package util

import (
	"crypto/x509"
	"time"
)

// CertHalfwayExpired returns true if half of the cert validity period has elapsed, false if not.
func CertHalfwayExpired(cert *x509.Certificate) bool {
	now := time.Now()
	halfValidPeriod := cert.NotAfter.Sub(cert.NotBefore).Nanoseconds() / 2
	halfExpiration := cert.NotBefore.Add(time.Duration(halfValidPeriod) * time.Nanosecond)
	return now.After(halfExpiration)
}
