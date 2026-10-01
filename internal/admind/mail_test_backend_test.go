package admind

import (
	"context"

	"github.com/yeomyeonggeori/internkim/internal/mail"

	"github.com/emersion/go-imap/v2"
)

type fakeMailBackend struct {
	testedAccount mail.Account
	listedAccount mail.Account
	listInput     mail.MessageListRequest
	sentAccount   mail.Account
	sentMessage   mail.MessageSendRequest
	movedUID      uint32
	movedTarget   string
	markedUID     uint32
	markedSeen    *bool
	markedFlagged *bool
	readUID       uint32
	mailboxes     []mail.MailboxResponse
	messages      []mail.MessageResponse
	messageDetail mail.MessageDetailResponse
}

func (backend *fakeMailBackend) TestAccount(ctx context.Context, account mail.Account) error {
	backend.testedAccount = account
	return nil
}

func (backend *fakeMailBackend) ListMailboxes(ctx context.Context, account mail.Account) ([]mail.MailboxResponse, error) {
	backend.listedAccount = account
	return backend.mailboxes, nil
}

func (backend *fakeMailBackend) ListMessages(ctx context.Context, account mail.Account, input mail.MessageListRequest) (mail.MessageListResponse, error) {
	backend.listInput = input
	uids := mailMessageResponseUIDs(backend.messages)
	visibleUIDs, hasMoreMessages := mail.VisibleMessageUIDs(uids, input.Limit)
	messages := backend.messages
	if hasMoreMessages {
		messages = messages[:input.Limit]
	}
	return mail.MessageListResponse{
		Messages:   messages,
		NextCursor: mail.NextMessageCursor(input, visibleUIDs, hasMoreMessages),
	}, nil
}

func (backend *fakeMailBackend) ReadMessage(ctx context.Context, account mail.Account, mailbox string, uid uint32) (mail.MessageDetailResponse, error) {
	backend.readUID = uid
	return backend.messageDetail, nil
}

func (backend *fakeMailBackend) SendMessage(ctx context.Context, account mail.Account, input mail.MessageSendRequest) (mail.SendResult, error) {
	backend.sentAccount = account
	backend.sentMessage = input
	return mail.SendResult{Sent: true, MessageID: "sent-1@example.com", AppendedTo: account.SentMailbox}, nil
}

func (backend *fakeMailBackend) MoveMessage(ctx context.Context, account mail.Account, mailbox string, uid uint32, targetMailbox string) error {
	backend.movedUID = uid
	backend.movedTarget = targetMailbox
	return nil
}

func (backend *fakeMailBackend) MarkMessage(ctx context.Context, account mail.Account, mailbox string, uid uint32, input mail.MessageMarkRequest) error {
	backend.markedUID = uid
	backend.markedSeen = input.Seen
	backend.markedFlagged = input.Flagged
	return nil
}

func mailMessageResponseUIDs(messages []mail.MessageResponse) []imap.UID {
	uids := make([]imap.UID, 0, len(messages))
	for _, message := range messages {
		uids = append(uids, imap.UID(message.UID))
	}
	return uids
}
