// Package delivery sends books to e-readers (Amazon Send-to-Kindle via SMTP).
package delivery

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string // must be on the Kindle account's approved sender list
}

// MaxAttachmentBytes mirrors Amazon's Send-to-Kindle email attachment limit.
const MaxAttachmentBytes = 50 << 20

func (c SMTPConfig) Validate() error {
	if c.Host == "" || c.Port == "" || c.From == "" {
		return errors.New("SMTP host, port and sender address must be configured")
	}
	if strings.ContainsAny(c.From+c.Host, "\r\n") {
		return errors.New("invalid SMTP configuration")
	}
	return nil
}

// SendFile emails filePath as an attachment to the given Kindle address.
// Port 465 uses implicit TLS; other ports use STARTTLS when offered.
func SendFile(c SMTPConfig, to, filePath string) error {
	c = c.Clean()
	if err := c.Validate(); err != nil {
		return err
	}
	if !strings.Contains(to, "@") || strings.ContainsAny(to, "\r\n") {
		return errors.New("invalid Kindle email address")
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("book file unavailable: %w", err)
	}
	if info.Size() > MaxAttachmentBytes {
		return errors.New("book file exceeds the 50MB Send-to-Kindle limit")
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	msg := buildMessage(c.From, to, filepath.Base(filePath), data)
	return send(c, to, msg)
}

func buildMessage(from, to, filename string, data []byte) []byte {
	boundary := "novelcheck-" + fmt.Sprint(time.Now().UnixNano())
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\nTo: %s\r\nSubject: NovelCheck delivery\r\n", from, to)
	fmt.Fprintf(&b, "Date: %s\r\nMIME-Version: 1.0\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Content-Type: multipart/mixed; boundary=%q\r\n\r\n", boundary)
	fmt.Fprintf(&b, "--%s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nSent from NovelCheck.\r\n", boundary)
	name := mime.QEncoding.Encode("utf-8", filename)
	fmt.Fprintf(&b, "--%s\r\nContent-Type: %s; name=%q\r\n", boundary, contentType(filename), name)
	fmt.Fprintf(&b, "Content-Transfer-Encoding: base64\r\nContent-Disposition: attachment; filename=%q\r\n\r\n", name)
	enc := base64.StdEncoding.EncodeToString(data)
	for len(enc) > 76 {
		b.WriteString(enc[:76] + "\r\n")
		enc = enc[76:]
	}
	b.WriteString(enc + "\r\n")
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return b.Bytes()
}

func contentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".epub":
		return "application/epub+zip"
	case ".pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}

func send(c SMTPConfig, to string, msg []byte) error {
	addr := net.JoinHostPort(c.Host, c.Port)
	tlsCfg := &tls.Config{ServerName: c.Host, MinVersion: tls.VersionTLS12}
	var conn net.Conn
	var err error
	dialer := &net.Dialer{Timeout: 20 * time.Second}
	if c.Port == "465" {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsCfg)
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return err
	}
	cl, err := smtp.NewClient(conn, c.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer cl.Close()
	if ok, _ := cl.Extension("STARTTLS"); ok && c.Port != "465" {
		if err := cl.StartTLS(tlsCfg); err != nil {
			return err
		}
	}
	if c.Username != "" {
		if err := cl.Auth(smtp.PlainAuth("", c.Username, c.Password, c.Host)); err != nil {
			return authError(c, err)
		}
	}
	if err := cl.Mail(c.From); err != nil {
		return err
	}
	if err := cl.Rcpt(to); err != nil {
		return err
	}
	wc, err := cl.Data()
	if err != nil {
		return err
	}
	if _, err := wc.Write(msg); err != nil {
		return err
	}
	if err := wc.Close(); err != nil {
		return err
	}
	return cl.Quit()
}

// CheckLogin connects to the SMTP server and signs in without sending mail.
func CheckLogin(c SMTPConfig) error {
	c = c.Clean()
	if err := c.Validate(); err != nil {
		return err
	}
	addr := net.JoinHostPort(c.Host, c.Port)
	tlsCfg := &tls.Config{ServerName: c.Host, MinVersion: tls.VersionTLS12}
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	var conn net.Conn
	var err error
	if c.Port == "465" {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsCfg)
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	cl, err := smtp.NewClient(conn, c.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer cl.Close()
	if ok, _ := cl.Extension("STARTTLS"); ok && c.Port != "465" {
		if err := cl.StartTLS(tlsCfg); err != nil {
			return err
		}
	}
	if c.Username != "" {
		if err := cl.Auth(smtp.PlainAuth("", c.Username, c.Password, c.Host)); err != nil {
			return authError(c, err)
		}
	}
	return cl.Quit()
}
