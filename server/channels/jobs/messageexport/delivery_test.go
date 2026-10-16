// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package messageexport

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCertificate(t *testing.T) tls.Certificate {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

type receivedMail struct {
	from, to, data, auth string
}

// fakeSMTPServer is a minimal SMTP server supporting STARTTLS and AUTH PLAIN.
type fakeSMTPServer struct {
	listener net.Listener
	cert     tls.Certificate
	noTLS    bool

	mut      sync.Mutex
	received []receivedMail
}

func newFakeSMTPServer(t *testing.T, noTLS bool) *fakeSMTPServer {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s := &fakeSMTPServer{listener: l, cert: testCertificate(t), noTLS: noTLS}
	t.Cleanup(func() { l.Close() })
	go s.serve()
	return s
}

func (s *fakeSMTPServer) port() string {
	return strconv.Itoa(s.listener.Addr().(*net.TCPAddr).Port)
}

func (s *fakeSMTPServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *fakeSMTPServer) handle(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	w := func(line string) { _, _ = conn.Write([]byte(line + "\r\n")) }
	w("220 127.0.0.1 ESMTP fake")
	tlsOn := false
	var cur receivedMail
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		cmd := strings.ToUpper(strings.SplitN(line, " ", 2)[0])
		switch cmd {
		case "EHLO", "HELO":
			if !tlsOn && !s.noTLS {
				w("250-127.0.0.1")
				w("250-STARTTLS")
				w("250 AUTH PLAIN LOGIN")
			} else {
				w("250-127.0.0.1")
				w("250 AUTH PLAIN LOGIN")
			}
		case "STARTTLS":
			w("220 ready")
			tlsConn := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{s.cert}})
			if err := tlsConn.Handshake(); err != nil {
				return
			}
			conn = tlsConn
			r = bufio.NewReader(conn)
			tlsOn = true
		case "AUTH":
			parts := strings.Fields(line)
			if len(parts) == 3 {
				decoded, _ := base64.StdEncoding.DecodeString(parts[2])
				cur.auth = string(decoded)
			}
			w("235 ok")
		case "MAIL":
			cur.from = strings.TrimSuffix(strings.TrimPrefix(line[len("MAIL FROM:"):], "<"), ">")
			w("250 ok")
		case "RCPT":
			cur.to = strings.TrimSuffix(strings.TrimPrefix(line[len("RCPT TO:"):], "<"), ">")
			w("250 ok")
		case "DATA":
			w("354 go ahead")
			var sb strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				sb.WriteString(l)
			}
			cur.data = sb.String()
			s.mut.Lock()
			s.received = append(s.received, cur)
			s.mut.Unlock()
			cur = receivedMail{auth: cur.auth}
			w("250 queued")
		case "RSET", "NOOP":
			w("250 ok")
		case "QUIT":
			w("221 bye")
			return
		default:
			w("502 unknown")
		}
	}
}

func TestSMTPSender(t *testing.T) {
	server := newFakeSMTPServer(t, false)
	s := &smtpSender{
		host:      "127.0.0.1",
		port:      server.port(),
		username:  "user",
		password:  "secret",
		timeout:   10 * time.Second,
		tlsConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
	}

	require.NoError(t, s.Send(context.Background(), "alice@example.com", "archive@example.com", []byte("Subject: one\r\n\r\nbody one\r\n")))
	require.NoError(t, s.Send(context.Background(), "bob@example.com", "archive@example.com", []byte("Subject: two\r\n\r\nbody two\r\n")))
	require.NoError(t, s.Close())

	require.Eventually(t, func() bool {
		server.mut.Lock()
		defer server.mut.Unlock()
		return len(server.received) == 2
	}, 5*time.Second, 10*time.Millisecond)

	server.mut.Lock()
	defer server.mut.Unlock()
	assert.Equal(t, "alice@example.com", server.received[0].from)
	assert.Equal(t, "archive@example.com", server.received[0].to)
	assert.Contains(t, server.received[0].data, "body one")
	assert.Equal(t, "\x00user\x00secret", server.received[0].auth)
	assert.Equal(t, "bob@example.com", server.received[1].from)
	assert.Contains(t, server.received[1].data, "body two")
}

func TestSMTPSenderRequiresTLS(t *testing.T) {
	server := newFakeSMTPServer(t, true)
	s := &smtpSender{host: "127.0.0.1", port: server.port(), username: "user", password: "secret", timeout: 5 * time.Second}
	err := s.Send(context.Background(), "alice@example.com", "archive@example.com", []byte("x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "STARTTLS")
}
