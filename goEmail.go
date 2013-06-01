// goEmail provides a simplified interface to net/smtp for sending
// formatted emails.
package goEmail

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"github.com/stephens2424/quotedPrintable"
	"net/smtp"
	"strings"
	"time"
)

// Email is the basic data type of the goEmail package.
type Email struct {
	To, Cc, Bcc   []string
	From, Subject string
	emailBodies   []emailBody
	encoder       TransferEncoder
}

// Creates a new email with the default transfer encoder (quoted printable).
func NewEmail() *Email {
	enc := quotedPrintable.NewEncoder()
	return NewEmailWithEncoder(enc)
}

// Creates a new email with a specified transfer encoder.
func NewEmailWithEncoder(enc TransferEncoder) *Email {
	return &Email{encoder: enc}
}

// TransferEncoder defines an interface required by Email to prepare for
// transfer over the wire. Examples include quoted printable, and with a
// bit of wrapping, base64.
type TransferEncoder interface {
	Encode(src []byte) []byte
	TransferEncodingType() string
}

// FormatMailbox accepts an email address and a name and formats
// a mailbox entry useful in email headers.
func FormatMailbox(address, name string) string {
	if name == "" {
		return address
	}
	return name + " <" + address + ">"
}

// Adds a recipient to the email
func (email *Email) AddRecipient(mailbox string) {
	email.To = append(email.To, mailbox)
}

// Adds a cc recipient to the email
func (email *Email) AddCc(mailbox string) {
	email.Cc = append(email.Cc, mailbox)
}

// Adds a bcc recipient to the email
func (email *Email) AddBcc(mailbox string) {
	email.Bcc = append(email.Bcc, mailbox)
}

type emailBody struct {
	mimeType, bodyText string
}

// Add a body of any mimetype to the email.
func (email *Email) AddBody(mimeType, body string) {
	email.emailBodies = append(email.emailBodies, emailBody{mimeType, body})
}

// Adds an HTML body to the email, using the utf-8 charset.
func (email *Email) AddHtmlBody(body string) {
	email.AddBody("text/html; charset=utf-8", body)
}

// Adds a plaintext body to the email, using the utf-8 charset.
func (email *Email) AddTextBody(body string) {
	email.AddBody("text/plain; charset=utf-8", body)
}

// MessageID constructs the message ID of an email. This implementation
// defines the message ID as the sha1 digest of the entire email object.
func (e *Email) MessageID() string {
	hasher := sha1.New()
	hasher.Write([]byte(fmt.Sprintf("%+v", e)))
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

// formattedEmail encapsulates a format. It is used internally to manage
// multiple mimetypes in an email body.
type formattedEmail struct {
	buffer   bytes.Buffer
	boundary string
	encoder  TransferEncoder
}

// String returns the formatted email's internal buffer as a string.
func (e *formattedEmail) String() string {
	return e.buffer.String()
}

// addHeader adds a header to a formatted email.
func (e *formattedEmail) addHeader(field, value string) {
	if value != "" {
		e.buffer.WriteString(foldString(78, fmt.Sprintf("%s: ", field), value))
	}
}

// addBody adds a body segment to a formatted email, including the
// necessary headers for the mimetype.
func (fEmail *formattedEmail) addBody(emailBody emailBody) {
	fEmail.buffer.WriteString(fEmail.boundary + "\r\n")
	fEmail.addHeader("Content-Type", emailBody.mimeType)
	fEmail.addHeader("Content-Transfer-Encoding", fEmail.encoder.TransferEncodingType())
	fEmail.buffer.WriteString("\r\n")

	encoded := fEmail.encoder.Encode([]byte(emailBody.bodyText))
	fEmail.buffer.Write(encoded)
	fEmail.buffer.WriteString("\r\n")
}

// foldString returns a string, folded by "\r\n" where it
// overlaps a maximum length.
func foldString(maxLength int, prefix, s string) string {
	var foldedBuffer bytes.Buffer
	lineBuffer := bytes.NewBufferString(prefix)
	lineLength := lineBuffer.Len()

	for _, word := range strings.Split(s, " ") {
		wordLength := len(word)
		if wordLength+lineLength+1 <= maxLength {
			lineBuffer.WriteString(word)
			lineBuffer.WriteString(" ")
			lineLength += wordLength + 1
		} else {
			foldedBuffer.Write(lineBuffer.Bytes())
			foldedBuffer.WriteString("\r\n ")
			lineBuffer.Reset()
			lineBuffer.WriteString(word)
			lineBuffer.WriteString(" ")
			lineLength = wordLength + 1
		}
	}
	foldedBuffer.Write(lineBuffer.Bytes())
	foldedBuffer.WriteString("\r\n")
	return foldedBuffer.String()
}

// Formats an email for sending, per RFC 5322. This implementation uses the
// quoted-printable wire encoding for body segments.
func (email *Email) Format() []byte {
	fEmail := formattedEmail{encoder: email.encoder}

	boundary := fmt.Sprintf("=_%s", email.MessageID())
	fEmail.boundary = "\r\n--" + boundary

	fEmail.addHeader("To", strings.Join(email.To, ", "))
	fEmail.addHeader("Cc", strings.Join(email.Cc, ", "))
	fEmail.addHeader("Bcc", strings.Join(email.Bcc, ", "))
	fEmail.addHeader("From", email.From)
	fEmail.addHeader("Subject", email.Subject)
	fEmail.addHeader("Date", time.Now().Format(time.RFC1123Z))
	fEmail.addHeader("Content-Type", fmt.Sprintf("multipart/alternative; boundary=\"%s\"", boundary))
	fEmail.addHeader("MIME-Version", "1.0")

	for _, body := range email.emailBodies {
		fEmail.addBody(body)
	}

	return fEmail.buffer.Bytes()
}

// Send the formatted email using the specified server and authentication.
func (email *Email) Send(addr string, a smtp.Auth) error {
	return smtp.SendMail(
		addr,
		a,
		email.From,
		email.To,
		email.Format())
}
