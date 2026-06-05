package admind

import (
	"crypto/tls"
	"errors"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/charset"
	messagemail "github.com/emersion/go-message/mail"
)

func (backend standardMailBackend) openIMAPClient(account mailAccount) (*imapclient.Client, error) {
	address := net.JoinHostPort(account.IMAPHost, strconv.Itoa(account.IMAPPort))
	options := &imapclient.Options{
		TLSConfig: &tls.Config{ServerName: account.IMAPHost},
		WordDecoder: &mime.WordDecoder{
			CharsetReader: charset.Reader,
		},
		Dialer: &net.Dialer{Timeout: 30 * time.Second},
	}
	var client *imapclient.Client
	var errorValue error
	switch account.IMAPSecurity {
	case mailSecurityTLS:
		client, errorValue = imapclient.DialTLS(address, options)
	case mailSecurityStartTLS:
		client, errorValue = imapclient.DialStartTLS(address, options)
	case mailSecurityNone:
		client, errorValue = imapclient.DialInsecure(address, options)
	default:
		errorValue = errors.New("imapSecurity must be tls, starttls, or none")
	}
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := client.Login(account.IMAPUsername, account.IMAPPassword).Wait(); errorValue != nil {
		client.Close()
		return nil, errorValue
	}
	return client, nil
}

func (backend standardMailBackend) openSMTPClient(account mailAccount) (*smtp.Client, error) {
	address := net.JoinHostPort(account.SMTPHost, strconv.Itoa(account.SMTPPort))
	dialer := net.Dialer{Timeout: 30 * time.Second}
	var client *smtp.Client
	var errorValue error
	switch account.SMTPSecurity {
	case mailSecurityTLS:
		connection, dialError := tls.DialWithDialer(&dialer, "tcp", address, &tls.Config{ServerName: account.SMTPHost})
		if dialError != nil {
			return nil, dialError
		}
		client, errorValue = smtp.NewClient(connection, account.SMTPHost)
	case mailSecurityStartTLS:
		connection, dialError := dialer.Dial("tcp", address)
		if dialError != nil {
			return nil, dialError
		}
		client, errorValue = smtp.NewClient(connection, account.SMTPHost)
		if errorValue == nil {
			errorValue = client.StartTLS(&tls.Config{ServerName: account.SMTPHost})
		}
	case mailSecurityNone:
		connection, dialError := dialer.Dial("tcp", address)
		if dialError != nil {
			return nil, dialError
		}
		client, errorValue = smtp.NewClient(connection, account.SMTPHost)
	default:
		errorValue = errors.New("smtpSecurity must be tls, starttls, or none")
	}
	if errorValue != nil {
		return nil, errorValue
	}
	auth := smtp.PlainAuth("", account.SMTPUsername, account.SMTPPassword, account.SMTPHost)
	if errorValue := client.Auth(auth); errorValue != nil {
		_ = client.Close()
		return nil, errorValue
	}
	return client, nil
}

func (backend standardMailBackend) sendSMTPMessage(account mailAccount, recipients []string, messageDocument []byte) error {
	client, errorValue := backend.openSMTPClient(account)
	if errorValue != nil {
		return errorValue
	}
	defer client.Close()
	fromAddress, errorValue := messagemail.ParseAddress(account.FromAddress)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := client.Mail(fromAddress.Address); errorValue != nil {
		return errorValue
	}
	for _, recipient := range recipients {
		recipientAddress, errorValue := messagemail.ParseAddress(recipient)
		if errorValue != nil {
			return errorValue
		}
		if errorValue := client.Rcpt(recipientAddress.Address); errorValue != nil {
			return errorValue
		}
	}
	writer, errorValue := client.Data()
	if errorValue != nil {
		return errorValue
	}
	if _, errorValue := writer.Write(messageDocument); errorValue != nil {
		_ = writer.Close()
		return errorValue
	}
	if errorValue := writer.Close(); errorValue != nil {
		return errorValue
	}
	return client.Quit()
}

func (backend standardMailBackend) appendSentMessage(account mailAccount, messageDocument []byte) error {
	imapClient, errorValue := backend.openIMAPClient(account)
	if errorValue != nil {
		return errorValue
	}
	defer closeIMAPClient(imapClient)
	appendCommand := imapClient.Append(account.SentMailbox, int64(len(messageDocument)), &imap.AppendOptions{
		Flags: []imap.Flag{imap.FlagSeen},
		Time:  time.Now(),
	})
	if _, errorValue := appendCommand.Write(messageDocument); errorValue != nil {
		_ = appendCommand.Close()
		return errorValue
	}
	if errorValue := appendCommand.Close(); errorValue != nil {
		return errorValue
	}
	_, errorValue = appendCommand.Wait()
	return errorValue
}

func closeIMAPClient(client *imapclient.Client) {
	if client == nil {
		return
	}
	_ = client.Logout().Wait()
	client.Close()
}
