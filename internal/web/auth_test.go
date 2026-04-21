package web

import (
	"net/http"
	"testing"
)

func TestIsPrivateAddr(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		// Loopback
		{"127.0.0.1", true},
		{"127.0.0.2", true},
		{"::1", true},
		// RFC-1918
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.0.1", true},
		{"192.168.255.255", true},
		// RFC-4193 ULA
		{"fc00::1", true},
		{"fd12:3456:789a::1", true},
		// Public
		{"1.2.3.4", false},
		{"8.8.8.8", false},
		{"172.32.0.1", false}, // just outside 172.16/12
		// Edge cases
		{"", false},
		{"not-an-ip", false},
	}
	for _, tt := range tests {
		got := isPrivateAddr(tt.addr)
		if got != tt.want {
			t.Errorf("isPrivateAddr(%q) = %v, want %v", tt.addr, got, tt.want)
		}
	}
}

func TestClientIP_TrustedProxy(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := clientIP(r); got != "1.2.3.4" {
		t.Errorf("got %q, want %q", got, "1.2.3.4")
	}
}

func TestClientIP_MultiValueXFF(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:5678"
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	if got := clientIP(r); got != "1.2.3.4" {
		t.Errorf("got %q, want %q (should return first XFF value)", got, "1.2.3.4")
	}
}

func TestClientIP_UntrustedProxy(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	r.RemoteAddr = "8.8.8.8:1234"
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := clientIP(r); got != "8.8.8.8" {
		t.Errorf("got %q, want %q (XFF from public IP should be ignored)", got, "8.8.8.8")
	}
}

func TestClientIP_NoXFF(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.50:9999"
	if got := clientIP(r); got != "192.168.1.50" {
		t.Errorf("got %q, want %q", got, "192.168.1.50")
	}
}

func TestRoleAtLeast(t *testing.T) {
	tests := []struct {
		role, minRole string
		want          bool
	}{
		{"admin", "admin", true},
		{"admin", "viewer", true},
		{"admin", "none", true},
		{"viewer", "viewer", true},
		{"viewer", "none", true},
		{"viewer", "admin", false},
		{"none", "none", true},
		{"none", "viewer", false},
		{"none", "admin", false},
		// Unknown role maps to 0 (same as "none")
		{"", "none", true},
		{"", "viewer", false},
		{"", "admin", false},
	}
	for _, tt := range tests {
		got := roleAtLeast(tt.role, tt.minRole)
		if got != tt.want {
			t.Errorf("roleAtLeast(%q, %q) = %v, want %v", tt.role, tt.minRole, got, tt.want)
		}
	}
}
