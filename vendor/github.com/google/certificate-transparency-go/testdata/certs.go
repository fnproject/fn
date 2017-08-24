package testdata

import (
	"encoding/hex"
)

const (
	// CACertPEM is a copy of the CA cert at test/testdata/ca-cert.pem.
	CACertPEM = "-----BEGIN CERTIFICATE-----\n" +
		"MIIC0DCCAjmgAwIBAgIBADANBgkqhkiG9w0BAQUFADBVMQswCQYDVQQGEwJHQjEk\n" +
		"MCIGA1UEChMbQ2VydGlmaWNhdGUgVHJhbnNwYXJlbmN5IENBMQ4wDAYDVQQIEwVX\n" +
		"YWxlczEQMA4GA1UEBxMHRXJ3IFdlbjAeFw0xMjA2MDEwMDAwMDBaFw0yMjA2MDEw\n" +
		"MDAwMDBaMFUxCzAJBgNVBAYTAkdCMSQwIgYDVQQKExtDZXJ0aWZpY2F0ZSBUcmFu\n" +
		"c3BhcmVuY3kgQ0ExDjAMBgNVBAgTBVdhbGVzMRAwDgYDVQQHEwdFcncgV2VuMIGf\n" +
		"MA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDVimhTYhCicRmTbneDIRgcKkATxtB7\n" +
		"jHbrkVfT0PtLO1FuzsvRyY2RxS90P6tjXVUJnNE6uvMa5UFEJFGnTHgW8iQ8+EjP\n" +
		"KDHM5nugSlojgZ88ujfmJNnDvbKZuDnd/iYx0ss6hPx7srXFL8/BT/9Ab1zURmnL\n" +
		"svfP34b7arnRsQIDAQABo4GvMIGsMB0GA1UdDgQWBBRfnYgNyHPmVNT4DdjmsMEk\n" +
		"tEfDVTB9BgNVHSMEdjB0gBRfnYgNyHPmVNT4DdjmsMEktEfDVaFZpFcwVTELMAkG\n" +
		"A1UEBhMCR0IxJDAiBgNVBAoTG0NlcnRpZmljYXRlIFRyYW5zcGFyZW5jeSBDQTEO\n" +
		"MAwGA1UECBMFV2FsZXMxEDAOBgNVBAcTB0VydyBXZW6CAQAwDAYDVR0TBAUwAwEB\n" +
		"/zANBgkqhkiG9w0BAQUFAAOBgQAGCMxKbWTyIF4UbASydvkrDvqUpdryOvw4BmBt\n" +
		"OZDQoeojPUApV2lGOwRmYef6HReZFSCa6i4Kd1F2QRIn18ADB8dHDmFYT9czQiRy\n" +
		"f1HWkLxHqd81TbD26yWVXeGJPE3VICskovPkQNJ0tU4b03YmnKliibduyqQQkOFP\n" +
		"OwqULg==\n" +
		"-----END CERTIFICATE-----"

	// TestCertPEM is a copy of the leaf certificate at test/testdata/test-cert.pem;
	// it is signed by CACertPEM.
	TestCertPEM = "-----BEGIN CERTIFICATE-----\n" +
		"MIICyjCCAjOgAwIBAgIBBjANBgkqhkiG9w0BAQUFADBVMQswCQYDVQQGEwJHQjEk\n" +
		"MCIGA1UEChMbQ2VydGlmaWNhdGUgVHJhbnNwYXJlbmN5IENBMQ4wDAYDVQQIEwVX\n" +
		"YWxlczEQMA4GA1UEBxMHRXJ3IFdlbjAeFw0xMjA2MDEwMDAwMDBaFw0yMjA2MDEw\n" +
		"MDAwMDBaMFIxCzAJBgNVBAYTAkdCMSEwHwYDVQQKExhDZXJ0aWZpY2F0ZSBUcmFu\n" +
		"c3BhcmVuY3kxDjAMBgNVBAgTBVdhbGVzMRAwDgYDVQQHEwdFcncgV2VuMIGfMA0G\n" +
		"CSqGSIb3DQEBAQUAA4GNADCBiQKBgQCx+jeTYRH4eS2iCBw/5BklAIUx3H8sZXvZ\n" +
		"4d5HBBYLTJ8Z1UraRHBATBxRNBuPH3U43d0o2aykg2n8VkbdzHYX+BaKrltB1DMx\n" +
		"/KLa38gE1XIIlJBh+e75AspHzojGROAA8G7uzKvcndL2iiLMsJ3Hbg28c1J3ZbGj\n" +
		"eoxnYlPcwQIDAQABo4GsMIGpMB0GA1UdDgQWBBRqDZgqO2LES20u9Om7egGqnLeY\n" +
		"4jB9BgNVHSMEdjB0gBRfnYgNyHPmVNT4DdjmsMEktEfDVaFZpFcwVTELMAkGA1UE\n" +
		"BhMCR0IxJDAiBgNVBAoTG0NlcnRpZmljYXRlIFRyYW5zcGFyZW5jeSBDQTEOMAwG\n" +
		"A1UECBMFV2FsZXMxEDAOBgNVBAcTB0VydyBXZW6CAQAwCQYDVR0TBAIwADANBgkq\n" +
		"hkiG9w0BAQUFAAOBgQAXHNhKrEFKmgMPIqrI9oiwgbJwm4SLTlURQGzXB/7QKFl6\n" +
		"n678Lu4peNYzqqwU7TI1GX2ofg9xuIdfGsnniygXSd3t0Afj7PUGRfjL9mclbNah\n" +
		"ZHteEyA7uFgt59Zpb2VtHGC5X0Vrf88zhXGQjxxpcn0kxPzNJJKVeVgU0drA5g==\n" +
		"-----END CERTIFICATE-----\n"

	// TestPreCertPEM is a copy of the pre-certificate from
	// test/testdata/test-embedded-pre-cert.pem, which is a pre-certificate
	// signed by CACertPEM.
	TestPreCertPEM = "-----BEGIN CERTIFICATE-----\n" +
		"MIIC3zCCAkigAwIBAgIBBzANBgkqhkiG9w0BAQUFADBVMQswCQYDVQQGEwJHQjEk\n" +
		"MCIGA1UEChMbQ2VydGlmaWNhdGUgVHJhbnNwYXJlbmN5IENBMQ4wDAYDVQQIEwVX\n" +
		"YWxlczEQMA4GA1UEBxMHRXJ3IFdlbjAeFw0xMjA2MDEwMDAwMDBaFw0yMjA2MDEw\n" +
		"MDAwMDBaMFIxCzAJBgNVBAYTAkdCMSEwHwYDVQQKExhDZXJ0aWZpY2F0ZSBUcmFu\n" +
		"c3BhcmVuY3kxDjAMBgNVBAgTBVdhbGVzMRAwDgYDVQQHEwdFcncgV2VuMIGfMA0G\n" +
		"CSqGSIb3DQEBAQUAA4GNADCBiQKBgQC+75jnwmh3rjhfdTJaDB0ym+3xj6r015a/\n" +
		"BH634c4VyVui+A7kWL19uG+KSyUhkaeb1wDDjpwDibRc1NyaEgqyHgy0HNDnKAWk\n" +
		"EM2cW9tdSSdyba8XEPYBhzd+olsaHjnu0LiBGdwVTcaPfajjDK8VijPmyVCfSgWw\n" +
		"FAn/Xdh+tQIDAQABo4HBMIG+MB0GA1UdDgQWBBQgMVQa8lwF/9hli2hDeU9ekDb3\n" +
		"tDB9BgNVHSMEdjB0gBRfnYgNyHPmVNT4DdjmsMEktEfDVaFZpFcwVTELMAkGA1UE\n" +
		"BhMCR0IxJDAiBgNVBAoTG0NlcnRpZmljYXRlIFRyYW5zcGFyZW5jeSBDQTEOMAwG\n" +
		"A1UECBMFV2FsZXMxEDAOBgNVBAcTB0VydyBXZW6CAQAwCQYDVR0TBAIwADATBgor\n" +
		"BgEEAdZ5AgQDAQH/BAIFADANBgkqhkiG9w0BAQUFAAOBgQACocOeAVr1Tf8CPDNg\n" +
		"h1//NDdVLx8JAb3CVDFfM3K3I/sV+87MTfRxoM5NjFRlXYSHl/soHj36u0YtLGhL\n" +
		"BW/qe2O0cP8WbjLURgY1s9K8bagkmyYw5x/DTwjyPdTuIo+PdPY9eGMR3QpYEUBf\n" +
		"kGzKLC0+6/yBmWTr2M98CIY/vg==\n" +
		"-----END CERTIFICATE-----\n"
)

var (
	// TestCertProof is a copy of test/testdata/test-cert.proof, holding a TLS-encoded
	// ct.SignedCertificateTimestamp.
	TestCertProof = dh("00df1c2ec11500945247a96168325ddc5c7959e8f7c6d388fc002e0bbd3f74d7" +
		"640000013ddb27ded900000403004730450220606e10ae5c2d5a1b0aed49dc49" +
		"37f48de71a4e9784e9c208dfbfe9ef536cf7f2022100beb29c72d7d06d61d06b" +
		"db38a069469aa86fe12e18bb7cc45689a2c0187ef5a5")

	// TestPreCertProof is a copy of test/testdata/test-embedded-pre-cert.proof, holding a
	// TLS-encoded ct.SignedCertificateTimestamp.
	TestPreCertProof = dh("00df1c2ec11500945247a96168325ddc5c7959e8f7c6d388fc002e0bbd3f74d7" +
		"640000013ddb27df9300000403004730450220482f6751af35dba65436be1fd6" +
		"640f3dbf9a41429495924530288fa3e5e23e06022100e4edc0db3ac572b1e2f5" +
		"e8ab6a680653987dcf41027dfeffa105519d89edbf08")
)

func dh(h string) []byte {
	r, err := hex.DecodeString(h)
	if err != nil {
		panic(err)
	}
	return r
}
