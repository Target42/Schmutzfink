package auth

import (
	"context"
	"crypto/tls"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
)

const ldapTimeout = 5 * time.Second

type Directory interface {
	Authenticate(ctx context.Context, username, password string) error
}

type LDAPConfig struct {
	URL                string
	BindTemplate       string
	StartTLS           bool
	InsecureSkipVerify bool
}

type LDAP struct {
	cfg LDAPConfig
}

func NewLDAP(cfg LDAPConfig) *LDAP {
	return &LDAP{cfg: cfg}
}

func DomainTemplate(domain string) string {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return "{username}"
	}
	if strings.Contains(domain, ".") {
		return "{username}@" + domain
	}
	return domain + `\{username}`
}

func BindIdentity(template, username string) string {
	username = strings.TrimSpace(username)
	if username == "" {
		return ""
	}
	if strings.Contains(username, "@") || strings.ContainsAny(username, `\`) {
		return username
	}
	if template == "" {
		return username
	}
	return strings.ReplaceAll(template, "{username}", username)
}

func (c *LDAP) Authenticate(ctx context.Context, username, password string) error {
	if username == "" || password == "" {
		return ErrInvalidCredentials
	}
	identity := BindIdentity(c.cfg.BindTemplate, username)
	if identity == "" {
		return ErrInvalidCredentials
	}

	dialer := &net.Dialer{Timeout: ldapTimeout}
	if deadline, ok := ctx.Deadline(); ok {
		dialer.Deadline = deadline
	}
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: c.cfg.InsecureSkipVerify}
	if host := ldapServerName(c.cfg.URL); host != "" && !c.cfg.InsecureSkipVerify {
		tlsCfg.ServerName = host
	}

	conn, err := ldap.DialURL(c.cfg.URL, ldap.DialWithDialer(dialer), ldap.DialWithTLSConfig(tlsCfg))
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetTimeout(ldapTimeout)

	if c.cfg.StartTLS && !strings.HasPrefix(strings.ToLower(c.cfg.URL), "ldaps:") {
		if err := conn.StartTLS(tlsCfg); err != nil {
			return err
		}
	}
	if err := conn.Bind(identity, password); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}

func ldapServerName(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
