// Package s3sign builds SigV4 presigned URLs for S3 GET requests, offline and with the
// standard library only: the secret key never leaves the machine.
package s3sign

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	algorithm       = "AWS4-HMAC-SHA256"
	service         = "s3"
	unsignedPayload = "UNSIGNED-PAYLOAD"
	timeFormat      = "20060102T150405Z"
	dateFormat      = "20060102"
)

// Request is what a presigned GET signs.
type Request struct {
	// Endpoint is the scheme and host (and port), e.g. https://eu.cubestorage.io.
	Endpoint string
	Region   string
	// Path is the decoded request path, e.g. "/photos/a/report.pdf". It is encoded per
	// segment as S3 does: slashes are kept, everything but A-Z a-z 0-9 - . _ ~ is %XX.
	Path      string
	AccessKey string
	SecretKey string
	Expires   time.Duration
	Now       time.Time
}

// PathStyle is the path of `key` in `bucket` on a path-style endpoint: "/bucket/key".
func PathStyle(bucket, key string) string {
	return "/" + bucket + "/" + key
}

// PresignGet returns the presigned GET URL. Only the host header is signed and the payload
// is UNSIGNED-PAYLOAD, like the AWS SDKs do for presigned URLs.
func PresignGet(r Request) (string, error) {
	u, err := url.Parse(r.Endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", fmt.Errorf("invalid endpoint %q: expected https://host[:port]", r.Endpoint)
	}
	seconds := int64(r.Expires / time.Second)
	if seconds < 1 {
		return "", fmt.Errorf("expiry must be at least one second")
	}
	host := canonicalHost(u)
	now := r.Now.UTC()
	amzDate := now.Format(timeFormat)
	scope := strings.Join([]string{now.Format(dateFormat), r.Region, service, "aws4_request"}, "/")

	query := map[string]string{
		"X-Amz-Algorithm":     algorithm,
		"X-Amz-Credential":    r.AccessKey + "/" + scope,
		"X-Amz-Date":          amzDate,
		"X-Amz-Expires":       fmt.Sprintf("%d", seconds),
		"X-Amz-SignedHeaders": "host",
	}
	canonicalQuery := encodeQuery(query)
	canonicalURI := strings.TrimSuffix(u.EscapedPath(), "/") + EncodePath(r.Path)

	canonicalRequest := strings.Join([]string{
		"GET",
		canonicalURI,
		canonicalQuery,
		"host:" + host + "\n",
		"host",
		unsignedPayload,
	}, "\n")
	stringToSign := strings.Join([]string{algorithm, amzDate, scope, hexSHA256(canonicalRequest)}, "\n")
	signature := hex.EncodeToString(hmacSHA256(signingKey(r.SecretKey, now, r.Region), stringToSign))

	return fmt.Sprintf("%s://%s%s?%s&X-Amz-Signature=%s", u.Scheme, u.Host, canonicalURI, canonicalQuery, signature), nil
}

// canonicalHost drops the scheme's default port, as clients do in the Host header.
func canonicalHost(u *url.URL) string {
	port := u.Port()
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		return u.Hostname()
	}
	return u.Host
}

func signingKey(secret string, now time.Time, region string) []byte {
	k := hmacSHA256([]byte("AWS4"+secret), now.Format(dateFormat))
	k = hmacSHA256(k, region)
	k = hmacSHA256(k, service)
	return hmacSHA256(k, "aws4_request")
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func hexSHA256(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func encodeQuery(q map[string]string) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, uriEncode(k, true)+"="+uriEncode(q[k], true))
	}
	return strings.Join(parts, "&")
}

// EncodePath encodes a decoded path for SigV4 and the URL: each segment on its own, the
// slashes kept and never collapsed.
func EncodePath(path string) string {
	return uriEncode(path, false)
}

// uriEncode is SigV4's UriEncode: unreserved characters stay, every other byte of the UTF-8
// encoding becomes %XX (uppercase). `encodeSlash` is false for paths.
func uriEncode(s string, encodeSlash bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case 'A' <= c && c <= 'Z', 'a' <= c && c <= 'z', '0' <= c && c <= '9', c == '-', c == '.', c == '_', c == '~':
			b.WriteByte(c)
		case c == '/' && !encodeSlash:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
