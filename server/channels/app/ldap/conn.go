// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package ldap

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/ldap"

	"github.com/mattermost/mattermost/server/public/model"
)

// errSizeLimitExceeded is returned by searches with an explicit size limit
// when more entries match.
var errSizeLimitExceeded = errors.New("more entries than requested match the filter")

// Conn is the subset of an LDAP connection used by this package. It is
// satisfied by *ldap.Conn and can be faked in tests.
type Conn interface {
	Bind(username, password string) error
	ExternalBind() error
	Search(req *ldap.SearchRequest) (*ldap.SearchResult, error)
	SearchWithPaging(req *ldap.SearchRequest, pagingSize uint32) (*ldap.SearchResult, error)
	Close()
}

// Dialer opens a new, unauthenticated, connection to the directory server.
type Dialer func(s *settings, getConfigFile func(name string) ([]byte, error)) (Conn, *model.AppError)

// dialLDAP is the production Dialer.
func dialLDAP(s *settings, getConfigFile func(name string) ([]byte, error)) (Conn, *model.AppError) {
	if s.Server == "" {
		return nil, model.NewAppError("ldap.dial", "ent.ldap.do_login.unable_to_connect.app_error", nil, "no AD/LDAP server configured", http.StatusInternalServerError)
	}

	tlsConfig, appErr := buildTLSConfig(s, getConfigFile)
	if appErr != nil {
		return nil, appErr
	}

	address := net.JoinHostPort(s.Server, strconv.Itoa(s.Port))
	dialer := &net.Dialer{Timeout: s.QueryTimeout}

	var (
		rawConn net.Conn
		err     error
	)
	switch s.ConnectionSecurity {
	case model.ConnSecurityTLS:
		rawConn, err = tls.DialWithDialer(dialer, "tcp", address, tlsConfig)
	default:
		rawConn, err = dialer.Dial("tcp", address)
	}
	if err != nil {
		return nil, model.NewAppError("ldap.dial", "ent.ldap.do_login.unable_to_connect.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	conn := ldap.NewConn(rawConn, s.ConnectionSecurity == model.ConnSecurityTLS)
	conn.Start()
	conn.SetTimeout(s.QueryTimeout)

	if s.ConnectionSecurity == model.ConnSecurityStarttls {
		if err := conn.StartTLS(tlsConfig); err != nil {
			conn.Close()
			return nil, model.NewAppError("ldap.dial", "ent.ldap.do_login.unable_to_connect.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
	}

	return conn, nil
}

// buildTLSConfig builds the TLS configuration used for TLS and STARTTLS
// connections, including the optional client certificate.
func buildTLSConfig(s *settings, getConfigFile func(name string) ([]byte, error)) (*tls.Config, *model.AppError) {
	cfg := &tls.Config{
		ServerName:         s.Server,
		InsecureSkipVerify: s.SkipCertificateVerification, //nolint:gosec // explicit admin setting
		MinVersion:         tls.VersionTLS12,
	}

	if s.PublicCertificateFile == "" || s.PrivateKeyFile == "" || getConfigFile == nil {
		return cfg, nil
	}

	certPEM, err := getConfigFile(s.PublicCertificateFile)
	if err != nil {
		return nil, model.NewAppError("ldap.buildTLSConfig", "ent.ldap.do_login.certificate.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	keyPEM, err := getConfigFile(s.PrivateKeyFile)
	if err != nil {
		return nil, model.NewAppError("ldap.buildTLSConfig", "ent.ldap.do_login.key.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, model.NewAppError("ldap.buildTLSConfig", "ent.ldap.do_login.x509.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	cfg.Certificates = []tls.Certificate{pair}
	return cfg, nil
}

// hasClientCertificate reports whether mutual TLS is configured.
func hasClientCertificate(s *settings) bool {
	return s.ConnectionSecurity != model.ConnSecurityNone && s.PublicCertificateFile != "" && s.PrivateKeyFile != ""
}

// session is an authenticated connection bound with the configured service
// account, along with the settings it was opened with.
type session struct {
	conn Conn
	s    *settings
}

// openSession connects to the server and binds with the service account.
func openSession(dial Dialer, s *settings, getConfigFile func(string) ([]byte, error)) (*session, *model.AppError) {
	conn, appErr := dial(s, getConfigFile)
	if appErr != nil {
		return nil, appErr
	}

	var err error
	switch {
	case s.BindUsername != "":
		err = conn.Bind(s.BindUsername, s.BindPassword)
	case hasClientCertificate(s):
		err = conn.ExternalBind()
	default:
		// Anonymous access: nothing to do, searches are sent unauthenticated.
	}
	if err != nil {
		conn.Close()
		return nil, model.NewAppError("ldap.openSession", "ent.ldap.do_login.bind_admin_user.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	return &session{conn: conn, s: s}, nil
}

func (ss *session) Close() {
	if ss != nil && ss.conn != nil {
		ss.conn.Close()
	}
}

func (ss *session) timeLimit() int {
	return int(ss.s.QueryTimeout / time.Second)
}

// search runs a subtree search below the base DN, using the paged results
// control when a maximum page size is configured.
func (ss *session) search(filter string, attributes []string) ([]*ldap.Entry, error) {
	return ss.searchBase(ss.s.BaseDN, ldap.ScopeWholeSubtree, filter, attributes, 0)
}

// searchLimit runs a search expected to return at most sizeLimit entries. It
// returns errSizeLimitExceeded when more entries match.
func (ss *session) searchLimit(filter string, attributes []string, sizeLimit int) ([]*ldap.Entry, error) {
	return ss.searchBase(ss.s.BaseDN, ldap.ScopeWholeSubtree, filter, attributes, sizeLimit)
}

func (ss *session) searchBase(base string, scope int, filter string, attributes []string, sizeLimit int) ([]*ldap.Entry, error) {
	req := ldap.NewSearchRequest(
		base,
		scope,
		ldap.NeverDerefAliases,
		sizeLimit,
		ss.timeLimit(),
		false,
		filter,
		attributes,
		nil,
	)

	var (
		res *ldap.SearchResult
		err error
	)
	if ss.s.MaxPageSize > 0 && sizeLimit == 0 && scope != ldap.ScopeBaseObject {
		res, err = ss.conn.SearchWithPaging(req, uint32(ss.s.MaxPageSize))
	} else {
		res, err = ss.conn.Search(req)
	}
	if err != nil {
		// More entries than explicitly requested exist.
		if sizeLimit > 0 && ldap.IsErrorWithCode(err, ldap.LDAPResultSizeLimitExceeded) {
			return nil, errSizeLimitExceeded
		}
		// Searching below a DN that does not exist simply returns nothing.
		if ldap.IsErrorWithCode(err, ldap.LDAPResultNoSuchObject) {
			return nil, nil
		}
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	return res.Entries, nil
}

// readEntry reads a single entry by DN. It returns nil if the entry does not
// exist or does not match the filter.
func (ss *session) readEntry(dn string, filter string, attributes []string) (*ldap.Entry, error) {
	if filter == "" {
		filter = "(objectClass=*)"
	}
	entries, err := ss.searchBase(dn, ldap.ScopeBaseObject, filter, attributes, 0)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}
	return entries[0], nil
}

// memberValues returns the member DNs of a group entry, transparently
// following Active Directory ranged attribute retrieval for large groups.
func (ss *session) memberValues(e *ldap.Entry) []string {
	var members []string
	for _, attrName := range groupMemberAttributes {
		base := strings.ToLower(attrName)
		for _, a := range e.Attributes {
			name := strings.ToLower(a.Name)
			if name == base {
				members = append(members, a.Values...)
				continue
			}
			if !strings.HasPrefix(name, base+";range=") {
				continue
			}
			members = append(members, a.Values...)
			members = append(members, ss.fetchRemainingRange(e.DN, attrName, name[len(base)+len(";range="):])...)
		}
	}
	return members
}

// fetchRemainingRange fetches the remaining values of a ranged attribute.
// rng is the range returned by the server, e.g. "0-1499" or "1500-*".
func (ss *session) fetchRemainingRange(dn, attr, rng string) []string {
	var values []string
	for range 10000 { // hard stop to protect against misbehaving servers
		parts := strings.SplitN(rng, "-", 2)
		if len(parts) != 2 || parts[1] == "*" {
			return values
		}
		end, err := strconv.Atoi(parts[1])
		if err != nil {
			return values
		}
		requested := fmt.Sprintf("%s;range=%d-*", attr, end+1)
		entry, err := ss.readEntry(dn, "", []string{requested})
		if err != nil || entry == nil {
			return values
		}
		found := false
		prefix := strings.ToLower(attr) + ";range="
		for _, a := range entry.Attributes {
			name := strings.ToLower(a.Name)
			if strings.HasPrefix(name, prefix) {
				values = append(values, a.Values...)
				rng = name[len(prefix):]
				found = true
				break
			}
		}
		if !found {
			return values
		}
	}
	return values
}

// searchError converts a search error into an AppError.
func searchError(where string, err error) *model.AppError {
	if ldap.IsErrorWithCode(err, ldap.LDAPResultSizeLimitExceeded) {
		return model.NewAppError(where, "ent.ldap.syncronize.search_failure_size_exceeded.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	var ldapErr *ldap.Error
	if errors.As(err, &ldapErr) && ldapErr.ResultCode == ldap.ErrorNetwork {
		return model.NewAppError(where, "ent.ldap.do_login.unable_to_connect.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return model.NewAppError(where, "ent.ldap.do_login.search_ldap_server.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
}
