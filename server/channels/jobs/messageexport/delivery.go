// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package messageexport

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/platform/shared/mail"
)

// Global Relay SMTP endpoint for the A10 customer type (the host name shown in
// the admin console's example). There is no built-in endpoint for A9: those
// deployments must use the CUSTOM customer type with the server provided
// during Global Relay provisioning.
var (
	globalRelayA10Server = "feeds.globalrelay.com"
	globalRelayPort      = "25"
)

// emlSender delivers raw EML messages.
type emlSender interface {
	Send(ctx context.Context, from, to string, eml []byte) error
	Close() error
}

// smtpSender delivers EMLs to an SMTP server with STARTTLS and
// authentication, reusing the connection between messages.
type smtpSender struct {
	host     string
	port     string
	username string
	password string
	timeout  time.Duration
	// tlsConfig allows tests to trust a test certificate.
	tlsConfig *tls.Config

	client *smtp.Client
	conn   net.Conn
}

// newGlobalRelaySender creates the SMTP sender for the Global Relay settings.
func newGlobalRelaySender(settings *model.GlobalRelayMessageExportSettings) (*smtpSender, error) {
	if settings == nil {
		return nil, errors.New("missing Global Relay settings")
	}
	s := &smtpSender{
		port:     globalRelayPort,
		username: model.SafeDereference(settings.SMTPUsername),
		password: model.SafeDereference(settings.SMTPPassword),
		timeout:  time.Duration(model.SafeDereference(settings.SMTPServerTimeout)) * time.Second,
	}
	switch model.SafeDereference(settings.CustomerType) {
	case model.GlobalrelayCustomerTypeA9:
		return nil, errors.New("no built-in SMTP server for the Global Relay A9 customer type; use the CUSTOM customer type with the server provided by Global Relay")
	case model.GlobalrelayCustomerTypeA10:
		s.host = globalRelayA10Server
	case model.GlobalrelayCustomerTypeCustom:
		s.host = model.SafeDereference(settings.CustomSMTPServerName)
		s.port = model.SafeDereference(settings.CustomSMTPPort)
	default:
		return nil, fmt.Errorf("unknown Global Relay customer type %q", model.SafeDereference(settings.CustomerType))
	}
	if s.host == "" || s.port == "" {
		return nil, errors.New("the Global Relay SMTP server is not configured")
	}
	if s.timeout <= 0 {
		s.timeout = 30 * time.Minute
	}
	return s, nil
}

func (s *smtpSender) connect(ctx context.Context) error {
	dialer := &net.Dialer{Timeout: s.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(s.host, s.port))
	if err != nil {
		return fmt.Errorf("unable to connect to the SMTP server: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(s.timeout))

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("unable to start the SMTP session: %w", err)
	}
	if hostname, herr := os.Hostname(); herr == nil && hostname != "" {
		if err := client.Hello(hostname); err != nil {
			client.Close()
			return fmt.Errorf("SMTP EHLO failed: %w", err)
		}
	}

	tlsConfig := s.tlsConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12}
	}
	if ok, _ := client.Extension("STARTTLS"); !ok {
		client.Close()
		return errors.New("the SMTP server does not support STARTTLS")
	}
	if err := client.StartTLS(tlsConfig); err != nil {
		client.Close()
		return fmt.Errorf("unable to start TLS: %w", err)
	}

	if s.username != "" {
		auth := mail.LoginAuth(s.username, s.password, s.host)
		if ok, mechs := client.Extension("AUTH"); ok && slices.Contains(strings.Fields(mechs), "PLAIN") {
			auth = smtp.PlainAuth("", s.username, s.password, s.host)
		}
		if err := client.Auth(auth); err != nil {
			client.Close()
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}

	s.client = client
	s.conn = conn
	return nil
}

func (s *smtpSender) send(from, to string, eml []byte) error {
	_ = s.conn.SetDeadline(time.Now().Add(s.timeout))
	if err := s.client.Mail(from); err != nil {
		return err
	}
	if err := s.client.Rcpt(to); err != nil {
		return err
	}
	w, err := s.client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(eml); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

// Send delivers one EML, (re)connecting when needed. A failure on an
// existing connection is retried once on a new connection.
func (s *smtpSender) Send(ctx context.Context, from, to string, eml []byte) error {
	for attempt := 0; ; attempt++ {
		if s.client == nil {
			if err := s.connect(ctx); err != nil {
				return err
			}
		}
		err := s.send(from, to, eml)
		if err == nil {
			return nil
		}
		s.reset()
		if attempt >= 1 {
			return err
		}
	}
}

func (s *smtpSender) reset() {
	if s.client != nil {
		s.client.Close()
	}
	s.client = nil
	s.conn = nil
}

// Close ends the SMTP session.
func (s *smtpSender) Close() error {
	if s.client == nil {
		return nil
	}
	err := s.client.Quit()
	s.reset()
	return err
}
