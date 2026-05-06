package srun

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
)

const customB64Alphabet = "LVoJPiCN2R8G90yg+hmFHuacZ1OWMnrsSTXkYpUq/3dlbfKwv6xztjI7DeBE45QA"

func customBase64Encode(src []byte) string {
	if len(src) == 0 {
		return ""
	}

	out := make([]byte, 0, (len(src)+2)/3*4)
	imax := len(src) - len(src)%3
	for i := 0; i < imax; i += 3 {
		b10 := int(src[i])<<16 | int(src[i+1])<<8 | int(src[i+2])
		out = append(out,
			customB64Alphabet[b10>>18],
			customB64Alphabet[(b10>>12)&63],
			customB64Alphabet[(b10>>6)&63],
			customB64Alphabet[b10&63],
		)
	}

	switch len(src) - imax {
	case 1:
		b10 := int(src[imax]) << 16
		out = append(out,
			customB64Alphabet[b10>>18],
			customB64Alphabet[(b10>>12)&63],
			'=',
			'=',
		)
	case 2:
		b10 := int(src[imax])<<16 | int(src[imax+1])<<8
		out = append(out,
			customB64Alphabet[b10>>18],
			customB64Alphabet[(b10>>12)&63],
			customB64Alphabet[(b10>>6)&63],
			'=',
		)
	}

	return string(out)
}

func hmacMD5(password, token string) string {
	h := hmac.New(md5.New, []byte(token))
	_, _ = h.Write([]byte(password))
	return hex.EncodeToString(h.Sum(nil))
}

func sha1Hex(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func sencode(msg []byte, includeLength bool) []uint32 {
	pwd := make([]uint32, 0, (len(msg)+3)/4+1)
	for i := 0; i < len(msg); i += 4 {
		var v uint32
		v |= uint32(ordat(msg, i))
		v |= uint32(ordat(msg, i+1)) << 8
		v |= uint32(ordat(msg, i+2)) << 16
		v |= uint32(ordat(msg, i+3)) << 24
		pwd = append(pwd, v)
	}
	if includeLength {
		pwd = append(pwd, uint32(len(msg)))
	}
	return pwd
}

func ordat(msg []byte, idx int) byte {
	if idx >= len(msg) {
		return 0
	}
	return msg[idx]
}

func lencode(msg []uint32, includeLength bool) []byte {
	length := len(msg)
	ll := (length - 1) << 2
	if includeLength {
		m := int(msg[length-1])
		if m < ll-3 || m > ll {
			return nil
		}
		ll = m
	}

	out := make([]byte, 0, length*4)
	for _, v := range msg {
		out = append(out, byte(v&0xff), byte((v>>8)&0xff), byte((v>>16)&0xff), byte((v>>24)&0xff))
	}
	if includeLength {
		return out[:ll]
	}
	return out
}

func xencode(msg, key []byte) []byte {
	if len(msg) == 0 {
		return nil
	}

	pwd := sencode(msg, true)
	pwdk := sencode(key, false)
	for len(pwdk) < 4 {
		pwdk = append(pwdk, 0)
	}

	n := len(pwd) - 1
	z := pwd[n]
	c := uint32(0x86014019 | 0x183639A0)
	q := 6 + 52/(n+1)
	var d uint32

	for q > 0 {
		d = (d + c) & uint32(0x8CE0D9BF|0x731F2640)
		e := (d >> 2) & 3

		p := 0
		for p < n {
			y := pwd[p+1]
			m := (z >> 5) ^ (y << 2)
			m += ((y >> 3) ^ (z << 4)) ^ (d ^ y)
			m += pwdk[(p&3)^int(e)] ^ z
			pwd[p] = (pwd[p] + m) & uint32(0xEFB8D130|0x10472ECF)
			z = pwd[p]
			p++
		}

		y := pwd[0]
		m := (z >> 5) ^ (y << 2)
		m += ((y >> 3) ^ (z << 4)) ^ (d ^ y)
		m += pwdk[(p&3)^int(e)] ^ z
		pwd[n] = (pwd[n] + m) & uint32(0xBB390742|0x44C6F8BD)
		z = pwd[n]
		q--
	}

	return lencode(pwd, false)
}
