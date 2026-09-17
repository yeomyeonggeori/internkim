package mail

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/emersion/go-imap/v2"
)

type Backend interface {
	TestAccount(ctx context.Context, account Account) error
	ListMailboxes(ctx context.Context, account Account) ([]MailboxResponse, error)
	ListMessages(ctx context.Context, account Account, input MessageListRequest) (MessageListResponse, error)
	ReadMessage(ctx context.Context, account Account, mailbox string, uid uint32) (MessageDetailResponse, error)
	SendMessage(ctx context.Context, account Account, input MessageSendRequest) (SendResult, error)
	MoveMessage(ctx context.Context, account Account, mailbox string, uid uint32, targetMailbox string) error
	MarkMessage(ctx context.Context, account Account, mailbox string, uid uint32, input MessageMarkRequest) error
}

type StandardBackend struct{}

func (backend StandardBackend) TestAccount(ctx context.Context, account Account) error {
	imapClient, errorValue := backend.openIMAPClient(account)
	if errorValue != nil {
		return fmt.Errorf("imap connection failed: %w", errorValue)
	}
	closeIMAPClient(imapClient)
	smtpClient, errorValue := backend.openSMTPClient(account)
	if errorValue != nil {
		return fmt.Errorf("smtp connection failed: %w", errorValue)
	}
	return smtpClient.Quit()
}

func (backend StandardBackend) ListMailboxes(ctx context.Context, account Account) ([]MailboxResponse, error) {
	imapClient, errorValue := backend.openIMAPClient(account)
	if errorValue != nil {
		return nil, errorValue
	}
	defer closeIMAPClient(imapClient)
	listData, errorValue := collectMailboxListData(ctx, imapClient)
	if errorValue != nil {
		return nil, errorValue
	}
	mailboxes := MailboxResponsesFromListData(listData)
	sort.SliceStable(mailboxes, func(firstIndex int, secondIndex int) bool {
		return MailboxSortKey(mailboxes[firstIndex].Name) < MailboxSortKey(mailboxes[secondIndex].Name)
	})
	return mailboxes, nil
}

func (backend StandardBackend) ListMessages(ctx context.Context, account Account, input MessageListRequest) (MessageListResponse, error) {
	imapClient, errorValue := backend.openIMAPClient(account)
	if errorValue != nil {
		return MessageListResponse{}, errorValue
	}
	defer closeIMAPClient(imapClient)
	selectedMailbox, errorValue := imapClient.Select(input.Mailbox, &imap.SelectOptions{ReadOnly: true}).Wait()
	if errorValue != nil {
		return MessageListResponse{}, errorValue
	}
	pageUIDs, errorValue := messageListUIDs(imapClient, selectedMailbox, input)
	if errorValue != nil {
		return MessageListResponse{}, errorValue
	}
	visibleUIDs, hasMoreMessages := VisibleMessageUIDs(pageUIDs, input.Limit)
	if len(visibleUIDs) == 0 {
		return MessageListResponse{Messages: []MessageResponse{}}, nil
	}
	fetchOptions := &imap.FetchOptions{
		UID:          true,
		Envelope:     true,
		Flags:        true,
		InternalDate: true,
	}
	messages, errorValue := imapClient.Fetch(imap.UIDSetNum(visibleUIDs...), fetchOptions).Collect()
	if errorValue != nil {
		return MessageListResponse{}, errorValue
	}
	responses := make([]MessageResponse, 0, len(messages))
	for _, message := range messages {
		response := messageResponseFromBuffer(input.Mailbox, message, nil)
		if response.UID != 0 {
			responses = append(responses, response)
		}
	}
	sort.SliceStable(responses, func(firstIndex int, secondIndex int) bool {
		return responses[firstIndex].UID > responses[secondIndex].UID
	})
	return MessageListResponse{
		Messages:    responses,
		NextCursor:  NextMessageCursor(input, visibleUIDs, hasMoreMessages),
		UIDNext:     uint32(selectedMailbox.UIDNext),
		UIDValidity: uint32(selectedMailbox.UIDValidity),
	}, nil
}

func (backend StandardBackend) ReadMessage(ctx context.Context, account Account, mailbox string, uid uint32) (MessageDetailResponse, error) {
	imapClient, errorValue := backend.openIMAPClient(account)
	if errorValue != nil {
		return MessageDetailResponse{}, errorValue
	}
	defer closeIMAPClient(imapClient)
	if _, errorValue := imapClient.Select(mailbox, &imap.SelectOptions{ReadOnly: true}).Wait(); errorValue != nil {
		return MessageDetailResponse{}, errorValue
	}
	bodySection := &imap.FetchItemBodySection{Peek: true}
	messages, errorValue := imapClient.Fetch(imap.UIDSetNum(imap.UID(uid)), &imap.FetchOptions{
		UID:          true,
		Envelope:     true,
		Flags:        true,
		InternalDate: true,
		BodySection:  []*imap.FetchItemBodySection{bodySection},
	}).Collect()
	if errorValue != nil {
		return MessageDetailResponse{}, errorValue
	}
	if len(messages) == 0 {
		return MessageDetailResponse{}, errors.New("message not found")
	}
	return messageDetailFromBuffer(mailbox, messages[0], bodySection), nil
}

func (backend StandardBackend) SendMessage(ctx context.Context, account Account, input MessageSendRequest) (SendResult, error) {
	messageDocument, recipients, errorValue := createMessageDocument(account, input)
	if errorValue != nil {
		return SendResult{}, errorValue
	}
	if errorValue := backend.SendSMTPMessage(account, recipients, messageDocument); errorValue != nil {
		return SendResult{}, errorValue
	}
	result := SendResult{Sent: true}
	if strings.TrimSpace(account.SentMailbox) == "" {
		return result, nil
	}
	if errorValue := backend.appendSentMessage(account, messageDocument); errorValue != nil {
		result.AppendWarning = errorValue.Error()
		return result, nil
	}
	result.AppendedTo = account.SentMailbox
	return result, nil
}

func (backend StandardBackend) MoveMessage(ctx context.Context, account Account, mailbox string, uid uint32, targetMailbox string) error {
	imapClient, errorValue := backend.openIMAPClient(account)
	if errorValue != nil {
		return errorValue
	}
	defer closeIMAPClient(imapClient)
	if _, errorValue := imapClient.Select(mailbox, nil).Wait(); errorValue != nil {
		return errorValue
	}
	_, errorValue = imapClient.Move(imap.UIDSetNum(imap.UID(uid)), targetMailbox).Wait()
	return errorValue
}

func (backend StandardBackend) MarkMessage(ctx context.Context, account Account, mailbox string, uid uint32, input MessageMarkRequest) error {
	imapClient, errorValue := backend.openIMAPClient(account)
	if errorValue != nil {
		return errorValue
	}
	defer closeIMAPClient(imapClient)
	if _, errorValue := imapClient.Select(mailbox, nil).Wait(); errorValue != nil {
		return errorValue
	}
	if input.Seen != nil {
		if errorValue := storeFlag(imapClient, uid, imap.FlagSeen, *input.Seen); errorValue != nil {
			return errorValue
		}
	}
	if input.Flagged != nil {
		return storeFlag(imapClient, uid, imap.FlagFlagged, *input.Flagged)
	}
	return nil
}
