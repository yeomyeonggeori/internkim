package admind

import (
	"context"

	"github.com/emersion/go-imap/v2"
)

type fakeMailBackend struct {
	testedAccount mailAccount
	listedAccount mailAccount
	listInput     mailMessageListRequest
	sentAccount   mailAccount
	sentMessage   mailMessageSendRequest
	movedUID      uint32
	movedTarget   string
	markedUID     uint32
	markedSeen    *bool
	markedFlagged *bool
	readUID       uint32
	mailboxes     []mailMailboxResponse
	messages      []mailMessageResponse
	messageDetail mailMessageDetailResponse
}

func (backend *fakeMailBackend) TestAccount(ctx context.Context, account mailAccount) error {
	backend.testedAccount = account
	return nil
}

func (backend *fakeMailBackend) ListMailboxes(ctx context.Context, account mailAccount) ([]mailMailboxResponse, error) {
	backend.listedAccount = account
	return backend.mailboxes, nil
}

func (backend *fakeMailBackend) ListMessages(ctx context.Context, account mailAccount, input mailMessageListRequest) (mailMessageListResponse, error) {
	backend.listInput = input
	uids := mailMessageResponseUIDs(backend.messages)
	visibleUIDs, hasMoreMessages := visibleMailMessageUIDs(uids, input.Limit)
	messages := backend.messages
	if hasMoreMessages {
		messages = messages[:input.Limit]
	}
	return mailMessageListResponse{
		Messages:   messages,
		NextCursor: nextMailMessageCursor(input, visibleUIDs, hasMoreMessages),
	}, nil
}

func (backend *fakeMailBackend) ReadMessage(ctx context.Context, account mailAccount, mailbox string, uid uint32) (mailMessageDetailResponse, error) {
	backend.readUID = uid
	return backend.messageDetail, nil
}

func (backend *fakeMailBackend) SendMessage(ctx context.Context, account mailAccount, input mailMessageSendRequest) (mailSendResult, error) {
	backend.sentAccount = account
	backend.sentMessage = input
	return mailSendResult{Sent: true, AppendedTo: account.SentMailbox}, nil
}

func (backend *fakeMailBackend) MoveMessage(ctx context.Context, account mailAccount, mailbox string, uid uint32, targetMailbox string) error {
	backend.movedUID = uid
	backend.movedTarget = targetMailbox
	return nil
}

func (backend *fakeMailBackend) MarkMessage(ctx context.Context, account mailAccount, mailbox string, uid uint32, input mailMessageMarkRequest) error {
	backend.markedUID = uid
	backend.markedSeen = input.Seen
	backend.markedFlagged = input.Flagged
	return nil
}

func mailMessageResponseUIDs(messages []mailMessageResponse) []imap.UID {
	uids := make([]imap.UID, 0, len(messages))
	for _, message := range messages {
		uids = append(uids, imap.UID(message.UID))
	}
	return uids
}
