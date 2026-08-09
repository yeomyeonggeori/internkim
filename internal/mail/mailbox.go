package mail

import (
	"context"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

type MailboxResponse struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Unseen      int    `json:"unseen"`
	Total       int    `json:"total"`
}

func collectMailboxListData(ctx context.Context, imapClient *imapclient.Client) ([]*imap.ListData, error) {
	listData, errorValue := imapClient.List("", "*", &imap.ListOptions{
		ReturnStatus: &imap.StatusOptions{NumMessages: true, NumUnseen: true},
	}).Collect()
	if errorValue == nil || ctx.Err() != nil {
		return listData, errorValue
	}
	return imapClient.List("", "*", nil).Collect()
}

func MailboxResponsesFromListData(listData []*imap.ListData) []MailboxResponse {
	mailboxes := make([]MailboxResponse, 0, len(listData))
	for _, mailboxData := range listData {
		if mailboxData == nil || mailboxData.Mailbox == "" || containsMailboxAttribute(mailboxData.Attrs, imap.MailboxAttrNoSelect) {
			continue
		}
		mailboxes = append(mailboxes, MailboxResponse{
			Name:        mailboxData.Mailbox,
			DisplayName: displayMailboxName(mailboxData),
			Unseen:      statusInteger(mailboxData.Status, "unseen"),
			Total:       statusInteger(mailboxData.Status, "total"),
		})
	}
	return mailboxes
}

func displayMailboxName(mailboxData *imap.ListData) string {
	if mailboxData == nil {
		return ""
	}
	for _, attribute := range mailboxData.Attrs {
		switch attribute {
		case imap.MailboxAttrSent:
			return "Sent"
		case imap.MailboxAttrDrafts:
			return "Drafts"
		case imap.MailboxAttrArchive:
			return "Archive"
		case imap.MailboxAttrTrash:
			return "Trash"
		case imap.MailboxAttrJunk:
			return "Junk"
		}
	}
	return mailboxData.Mailbox
}

func MailboxSortKey(mailbox string) string {
	normalizedMailbox := strings.ToLower(mailbox)
	switch {
	case normalizedMailbox == "inbox":
		return "00-" + normalizedMailbox
	case strings.Contains(normalizedMailbox, "sent"):
		return "10-" + normalizedMailbox
	case strings.Contains(normalizedMailbox, "draft"):
		return "20-" + normalizedMailbox
	case strings.Contains(normalizedMailbox, "archive"):
		return "30-" + normalizedMailbox
	case strings.Contains(normalizedMailbox, "trash"):
		return "40-" + normalizedMailbox
	default:
		return "50-" + normalizedMailbox
	}
}

func statusInteger(status *imap.StatusData, key string) int {
	if status == nil {
		return 0
	}
	switch key {
	case "unseen":
		if status.NumUnseen == nil {
			return 0
		}
		return int(*status.NumUnseen)
	default:
		if status.NumMessages == nil {
			return 0
		}
		return int(*status.NumMessages)
	}
}
