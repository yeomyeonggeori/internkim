package admind

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/emersion/go-imap/v2"
)

type mailBackend interface {
	TestAccount(ctx context.Context, account mailAccount) error
	ListMailboxes(ctx context.Context, account mailAccount) ([]mailMailboxResponse, error)
	ListMessages(ctx context.Context, account mailAccount, input mailMessageListRequest) (mailMessageListResponse, error)
	ReadMessage(ctx context.Context, account mailAccount, mailbox string, uid uint32) (mailMessageDetailResponse, error)
	SendMessage(ctx context.Context, account mailAccount, input mailMessageSendRequest) (mailSendResult, error)
	MoveMessage(ctx context.Context, account mailAccount, mailbox string, uid uint32, targetMailbox string) error
	MarkMessage(ctx context.Context, account mailAccount, mailbox string, uid uint32, input mailMessageMarkRequest) error
}

type standardMailBackend struct{}

func (backend standardMailBackend) TestAccount(ctx context.Context, account mailAccount) error {
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

func (backend standardMailBackend) ListMailboxes(ctx context.Context, account mailAccount) ([]mailMailboxResponse, error) {
	imapClient, errorValue := backend.openIMAPClient(account)
	if errorValue != nil {
		return nil, errorValue
	}
	defer closeIMAPClient(imapClient)
	listData, errorValue := collectMailMailboxListData(ctx, imapClient)
	if errorValue != nil {
		return nil, errorValue
	}
	mailboxes := mailMailboxResponsesFromListData(listData)
	sort.SliceStable(mailboxes, func(firstIndex int, secondIndex int) bool {
		return mailMailboxSortKey(mailboxes[firstIndex].Name) < mailMailboxSortKey(mailboxes[secondIndex].Name)
	})
	return mailboxes, nil
}

func (backend standardMailBackend) ListMessages(ctx context.Context, account mailAccount, input mailMessageListRequest) (mailMessageListResponse, error) {
	imapClient, errorValue := backend.openIMAPClient(account)
	if errorValue != nil {
		return mailMessageListResponse{}, errorValue
	}
	defer closeIMAPClient(imapClient)
	selectedMailbox, errorValue := imapClient.Select(input.Mailbox, &imap.SelectOptions{ReadOnly: true}).Wait()
	if errorValue != nil {
		return mailMessageListResponse{}, errorValue
	}
	pageUIDs, errorValue := messageListUIDs(imapClient, selectedMailbox, input)
	if errorValue != nil {
		return mailMessageListResponse{}, errorValue
	}
	visibleUIDs, hasMoreMessages := visibleMailMessageUIDs(pageUIDs, input.Limit)
	if len(visibleUIDs) == 0 {
		return mailMessageListResponse{}, nil
	}
	fetchOptions := &imap.FetchOptions{
		UID:          true,
		Envelope:     true,
		Flags:        true,
		InternalDate: true,
	}
	messages, errorValue := imapClient.Fetch(imap.UIDSetNum(visibleUIDs...), fetchOptions).Collect()
	if errorValue != nil {
		return mailMessageListResponse{}, errorValue
	}
	responses := make([]mailMessageResponse, 0, len(messages))
	for _, message := range messages {
		response := mailMessageResponseFromBuffer(input.Mailbox, message, nil)
		if response.UID != 0 {
			responses = append(responses, response)
		}
	}
	sort.SliceStable(responses, func(firstIndex int, secondIndex int) bool {
		return responses[firstIndex].UID > responses[secondIndex].UID
	})
	return mailMessageListResponse{
		Messages:    responses,
		NextCursor:  nextMailMessageCursor(input, visibleUIDs, hasMoreMessages),
		UIDNext:     uint32(selectedMailbox.UIDNext),
		UIDValidity: uint32(selectedMailbox.UIDValidity),
	}, nil
}

func (backend standardMailBackend) ReadMessage(ctx context.Context, account mailAccount, mailbox string, uid uint32) (mailMessageDetailResponse, error) {
	imapClient, errorValue := backend.openIMAPClient(account)
	if errorValue != nil {
		return mailMessageDetailResponse{}, errorValue
	}
	defer closeIMAPClient(imapClient)
	if _, errorValue := imapClient.Select(mailbox, &imap.SelectOptions{ReadOnly: true}).Wait(); errorValue != nil {
		return mailMessageDetailResponse{}, errorValue
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
		return mailMessageDetailResponse{}, errorValue
	}
	if len(messages) == 0 {
		return mailMessageDetailResponse{}, errors.New("message not found")
	}
	return mailMessageDetailFromBuffer(mailbox, messages[0], bodySection), nil
}

func (backend standardMailBackend) SendMessage(ctx context.Context, account mailAccount, input mailMessageSendRequest) (mailSendResult, error) {
	messageDocument, recipients, errorValue := createMailMessageDocument(account, input)
	if errorValue != nil {
		return mailSendResult{}, errorValue
	}
	if errorValue := backend.sendSMTPMessage(account, recipients, messageDocument); errorValue != nil {
		return mailSendResult{}, errorValue
	}
	result := mailSendResult{Sent: true}
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

func (backend standardMailBackend) MoveMessage(ctx context.Context, account mailAccount, mailbox string, uid uint32, targetMailbox string) error {
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

func (backend standardMailBackend) MarkMessage(ctx context.Context, account mailAccount, mailbox string, uid uint32, input mailMessageMarkRequest) error {
	imapClient, errorValue := backend.openIMAPClient(account)
	if errorValue != nil {
		return errorValue
	}
	defer closeIMAPClient(imapClient)
	if _, errorValue := imapClient.Select(mailbox, nil).Wait(); errorValue != nil {
		return errorValue
	}
	if input.Seen != nil {
		if errorValue := storeMailFlag(imapClient, uid, imap.FlagSeen, *input.Seen); errorValue != nil {
			return errorValue
		}
	}
	if input.Flagged != nil {
		return storeMailFlag(imapClient, uid, imap.FlagFlagged, *input.Flagged)
	}
	return nil
}
