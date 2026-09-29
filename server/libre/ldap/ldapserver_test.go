// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package ldap

import (
	"errors"
	"net"
	"sync"
	"testing"

	ber "github.com/go-asn1-ber/asn1-ber"
	"github.com/mattermost/ldap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLDAPServer is a minimal LDAPv3 server speaking the wire protocol, backed
// by a fakeDir. It supports simple binds and searches, which is enough to
// exercise the real client code path used in production.
type testLDAPServer struct {
	dir      *fakeDir
	listener net.Listener
	wg       sync.WaitGroup

	mu       sync.Mutex
	controls []string // control types received with searches
}

func startTestLDAPServer(t *testing.T, dir *fakeDir) *testLDAPServer {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &testLDAPServer{dir: dir, listener: l}
	srv.wg.Add(1)
	go srv.serve()
	t.Cleanup(func() {
		l.Close()
		srv.wg.Wait()
	})
	return srv
}

func (srv *testLDAPServer) port() int {
	return srv.listener.Addr().(*net.TCPAddr).Port
}

func (srv *testLDAPServer) serve() {
	defer srv.wg.Done()
	for {
		conn, err := srv.listener.Accept()
		if err != nil {
			return
		}
		srv.wg.Add(1)
		go func() {
			defer srv.wg.Done()
			defer conn.Close()
			srv.handle(conn)
		}()
	}
}

func ldapResult(tag ber.Tag, code int64) *ber.Packet {
	res := ber.Encode(ber.ClassApplication, ber.TypeConstructed, tag, nil, "result")
	res.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagEnumerated, code, "resultCode"))
	res.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "", "matchedDN"))
	res.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "", "diagnosticMessage"))
	return res
}

func envelope(id int64, op *ber.Packet) *ber.Packet {
	p := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "LDAP Response")
	p.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagInteger, id, "MessageID"))
	p.AppendChild(op)
	return p
}

func (srv *testLDAPServer) handle(conn net.Conn) {
	fc := &fakeConn{d: srv.dir}
	for {
		packet, err := ber.ReadPacket(conn)
		if err != nil || len(packet.Children) < 2 {
			return
		}
		id, _ := packet.Children[0].Value.(int64)
		op := packet.Children[1]

		switch op.Tag {
		case 0: // bind
			name := packetString(op.Children[1])
			password := packetString(op.Children[2])
			code := int64(0)
			if err := fc.Bind(name, password); err != nil {
				code = ldap.LDAPResultInvalidCredentials
			}
			conn.Write(envelope(id, ldapResult(1, code)).Bytes())
		case 2: // unbind
			return
		case 3: // search
			if len(packet.Children) > 2 {
				srv.mu.Lock()
				for _, c := range packet.Children[2].Children {
					if len(c.Children) > 0 {
						srv.controls = append(srv.controls, packetString(c.Children[0]))
					}
				}
				srv.mu.Unlock()
			}
			filter, err := ldap.DecompileFilter(op.Children[6])
			if err != nil {
				conn.Write(envelope(id, ldapResult(5, ldap.LDAPResultProtocolError)).Bytes())
				continue
			}
			scope, _ := op.Children[1].Value.(int64)
			sizeLimit, _ := op.Children[3].Value.(int64)
			var attrs []string
			for _, a := range op.Children[7].Children {
				attrs = append(attrs, packetString(a))
			}
			res, err := fc.Search(ldap.NewSearchRequest(packetString(op.Children[0]), int(scope), 0, int(sizeLimit), 0, false, filter, attrs, nil))
			if err != nil {
				code := int64(ldap.LDAPResultOther)
				var ldapErr *ldap.Error
				if errors.As(err, &ldapErr) {
					code = int64(ldapErr.ResultCode)
				}
				conn.Write(envelope(id, ldapResult(5, code)).Bytes())
				continue
			}
			for _, e := range res.Entries {
				entry := ber.Encode(ber.ClassApplication, ber.TypeConstructed, 4, nil, "entry")
				entry.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, e.DN, "dn"))
				attributes := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "attributes")
				for _, a := range e.Attributes {
					attr := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "attribute")
					attr.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, a.Name, "type"))
					vals := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSet, nil, "values")
					for _, v := range a.ByteValues {
						vals.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, string(v), "value"))
					}
					attr.AppendChild(vals)
					attributes.AppendChild(attr)
				}
				entry.AppendChild(attributes)
				conn.Write(envelope(id, entry).Bytes())
			}
			conn.Write(envelope(id, ldapResult(5, 0)).Bytes())
		default:
			conn.Write(envelope(id, ldapResult(op.Tag+1, ldap.LDAPResultUnwillingToPerform)).Bytes())
		}
	}
}

func TestWireProtocol(t *testing.T) {
	env := setupEnv(t)
	guid := []byte{0x78, 0x56, 0x34, 0x12, 0x34, 0x12, 0x78, 0x56, 0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0}
	env.dir.get("uid=alice,ou=people,dc=example,dc=com").set("objectGUID", guid)
	srv := startTestLDAPServer(t, env.dir)

	ls := &env.b.cfg.LdapSettings
	*ls.LdapServer = "127.0.0.1"
	*ls.LdapPort = srv.port()
	*ls.QueryTimeout = 5
	*ls.MaxPageSize = 100
	*ls.IdAttribute = "objectGUID"

	l := newLdap(env.b, nil) // real dialer and client

	users, appErr := l.GetAllLdapUsers(env.b.rctx())
	require.Nil(t, appErr)
	require.Len(t, users, 1, "only alice has an objectGUID")
	assert.Equal(t, "12345678-1234-5678-1234-56789abcdef0", *users[0].AuthData)
	assert.Contains(t, srv.controls, ldap.ControlTypePaging)

	user, appErr := l.DoLogin(env.b.rctx(), "12345678-1234-5678-1234-56789abcdef0", "alice-pwd")
	require.Nil(t, appErr)
	assert.Equal(t, "alice", user.Username)

	_, appErr = l.DoLogin(env.b.rctx(), "12345678-1234-5678-1234-56789abcdef0", "wrong")
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.ldap.do_login.invalid_password.app_error", appErr.Id)

	d := newDiagnostic(env.b, nil)
	require.Nil(t, d.RunTest(env.b.rctx()))

	submitted := env.b.cfg.LdapSettings
	submitted.BindPassword = new("wrong")
	appErr = d.RunTestConnection(env.b.rctx(), submitted)
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.ldap.connection.test_failed", appErr.Id)

	*ls.BaseDN = "ou=missing,dc=example,dc=com"
	users, appErr = l.GetAllLdapUsers(env.b.rctx())
	require.Nil(t, appErr)
	assert.Empty(t, users)
}
