package api

import (
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// "Use my SMTP email account" for the BookBub mailbox (Admin → E-readers & Delivery → Free book events): when the
// Send-to-Kindle email account also receives BookBub's emails, one press copies its address and app password
// (they stay on the server) into the BookBub mailbox, with the matching IMAP server, and switches the BookBub
// event on, instead of typing them in again.

// imapHostFor is the IMAP server that goes with an SMTP server ("smtp.gmail.com" → "imap.gmail.com:993"); "" when
// it can't be told.
func imapHostFor(smtp string) string {
	h := strings.ToLower(strings.TrimSpace(smtp))
	if i := strings.Index(h, ":"); i >= 0 {
		h = h[:i]
	}
	switch h {
	case "":
		return ""
	case "smtp.office365.com", "smtp-mail.outlook.com":
		return "outlook.office365.com:993"
	case "smtp.mail.yahoo.com":
		return "imap.mail.yahoo.com:993"
	case "smtp.mail.me.com":
		return "imap.mail.me.com:993"
	}
	if rest, ok := strings.CutPrefix(h, "smtp."); ok && strings.Contains(rest, ".") {
		return "imap." + rest + ":993"
	}
	return ""
}

// handleEventsMailUseSMTP: POST /api/admin/events-mail/use-smtp.
func (s *Server) handleEventsMailUseSMTP(w http.ResponseWriter, r *http.Request) {
	user, pass := s.Store.Setting(store.KeySMTPUser), s.Store.Setting(store.KeySMTPPassword)
	if user == "" || pass == "" {
		writeErr(w, http.StatusBadRequest, "Fill in and Save the SMTP username and password above first")
		return
	}
	host := imapHostFor(s.Store.Setting(store.KeySMTPHost))
	if host == "" {
		host = s.Store.Setting(store.KeyEventsMailHost) // not told from the SMTP host: keep what is there
	}
	if err := s.Store.SetSettings(map[string]string{
		store.KeyEventsMailUser: user, store.KeyEventsMailPassword: pass, store.KeyEventsMailHost: host, store.KeyEventsAutoBookBub: "true",
	}); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"user": user, "host": host})
}
