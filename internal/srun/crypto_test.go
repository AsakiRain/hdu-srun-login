package srun

import (
	"encoding/hex"
	"testing"
)

func TestCryptoMatchesPythonImplementation(t *testing.T) {
	tests := []struct {
		msg string
		key string
		hex string
		b64 string
	}{
		{"", "token", "", ""},
		{"hello", "token", "bb00d00c966e78bfb80ef138", "KvJ+JR1KrGQDJwPD"},
		{
			`{"username": "u", "password": "p", "ip": "1.2.3.4", "acid": "0", "enc_ver": "srun_bx1"}`,
			"abcdef123456",
			"33d5d8726d5b25e3ec1e270050a764af789f01a3af6c49f2d9ac310b157aea88c482c4919a14bd20d75eae6e3bd8b247515f4586ec07c57a767ee598642148f3c39033335b8f0e032d462b77f8995a5fc9084bd1a2cf60072e8f5e60",
			"95cZMUjWRrgbNXMLH8nYl7XsLOywWPqxIOv6o6uBB/kPSbmhUTm520nrlUDEIGRNHu5iTKvN6c3IsKaZ1oi244y+9z0WkvDJGHZlnAX1apARoPwh/b5SVxBgcUL=",
		},
	}

	for _, tt := range tests {
		got := xencode([]byte(tt.msg), []byte(tt.key))
		if hex.EncodeToString(got) != tt.hex {
			t.Fatalf("xencode(%q, %q) hex = %s", tt.msg, tt.key, hex.EncodeToString(got))
		}
		if customBase64Encode(got) != tt.b64 {
			t.Fatalf("b64(xencode(%q, %q)) = %s", tt.msg, tt.key, customBase64Encode(got))
		}
	}
}

func TestHMACMD5(t *testing.T) {
	got := hmacMD5("password", "token")
	want := "fd07c716fa2b6a86413916082c8bfdc3"
	if got != want {
		t.Fatalf("hmacMD5 = %s, want %s", got, want)
	}
}
