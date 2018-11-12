package util

import (
	"crypto/x509"
	"testing"
	"time"
)

func TestCertHalfwayExpired(t *testing.T) {
	tests := map[string]struct {
		testCert *x509.Certificate
		expected bool
	}{
		"expired now": {
			testCert: &x509.Certificate{
				NotBefore: time.Now().AddDate(0, 0, -1),
				NotAfter:  time.Now(),
			},
			expected: true,
		},
		"time left": {
			testCert: &x509.Certificate{
				NotBefore: time.Now().AddDate(0, 0, -1),
				NotAfter:  time.Now().AddDate(0, 0, 2),
			},
			expected: false,
		},
		"time up": {
			testCert: &x509.Certificate{
				NotBefore: time.Now().AddDate(0, 0, -2),
				NotAfter:  time.Now().AddDate(0, 0, 1),
			},
			expected: true,
		},
	}
	for name, tc := range tests {
		if CertHalfwayExpired(tc.testCert) != tc.expected {
			t.Errorf("%s: unexpected result, expected %v, got %v", name, tc.expected, !tc.expected)
		}
	}
}
