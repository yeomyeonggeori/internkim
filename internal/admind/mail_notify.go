package admind

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/centralplane"
	"github.com/yeomyeonggeori/internkim/internal/mail"
)

const (
	mailNotifyInterval = 2 * time.Minute
	mailNotifyBatch    = 20
	mailNotifyLongest  = 80
)

func (service *Service) keepMailAnnounced(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(mailNotifyInterval):
		}
		service.announceMailOnce(ctx)
	}
}

func (service *Service) announceMailOnce(ctx context.Context) {
	client := service.centralPlane()
	if client == nil {
		return
	}
	accounts, errorValue := client.MailAccounts(ctx)
	if errorValue != nil {
		log.Printf("mail notify: the accounts are unreadable: %v", errorValue)
		return
	}
	for _, held := range accounts {
		account, errorValue := service.mailPasswords(accountOfRecord(held.ActorEmail, held), held.MemberID)
		if errorValue != nil {
			log.Printf("mail notify: %v", errorValue)
			continue
		}
		service.announceMailFor(ctx, client, account)
	}
}

func (service *Service) announceMailFor(ctx context.Context, client *centralplane.Client, account mail.Account) {
	actorEmail := account.ActorEmail
	account = mail.NormalizeAccount(account)
	answered, errorValue := service.mailBackend.ListMessages(ctx, account, mail.MessageListRequest{
		Mailbox: account.DefaultMailbox,
		Limit:   mailNotifyBatch,
	})
	if errorValue != nil {
		log.Printf("mail notify: %s inbox was not read: %v", actorEmail, errorValue)
		return
	}

	seenUpTo, marked, errorValue := service.readMailNotifyMark(ctx, actorEmail)
	if errorValue != nil {
		log.Printf("mail notify: the mark for %s is unreadable: %v", actorEmail, errorValue)
		return
	}
	newest := mailNotifyNewestUID(answered.Messages)
	if !marked {
		if errorValue := service.writeMailNotifyMark(ctx, actorEmail, newest); errorValue != nil {
			log.Printf("mail notify: the first mark for %s was not kept: %v", actorEmail, errorValue)
		}
		return
	}

	arrived := mailNotifyArrivedSince(answered.Messages, seenUpTo)
	if len(arrived) == 0 {
		return
	}
	result, errorValue := client.Notify(ctx, mailNotifyNotification(arrived, actorEmail))
	if errorValue != nil {
		log.Printf("mail notify: %s was not told: %v", actorEmail, errorValue)
		return
	}
	log.Printf("mail notify: %s has %d newer than %d; told=%d reached=%d",
		actorEmail, len(arrived), seenUpTo, result.Told, result.Reached)
	if errorValue := service.writeMailNotifyMark(ctx, actorEmail, newest); errorValue != nil {
		log.Printf("mail notify: the mark for %s did not move: %v", actorEmail, errorValue)
	}
}

func mailNotifyNotification(arrived []mail.MessageResponse, actorEmail string) centralplane.Notification {
	newest := arrived[0]
	title := firstNonEmpty(newest.From, "메일")
	if len(arrived) > 1 {
		title = title + " 외 " + strconv.Itoa(len(arrived)-1) + "명"
	}
	return centralplane.Notification{
		Emails:   []string{actorEmail},
		Category: "mail",
		Title:    title,
		Body:     mailNotifyExcerpt(newest.Subject),
		OpenPath: "/mail/",
		Tag:      "mail-" + strconv.FormatUint(uint64(newest.UID), 10),
	}
}

func mailNotifyArrivedSince(messages []mail.MessageResponse, seenUpTo uint32) []mail.MessageResponse {
	arrived := make([]mail.MessageResponse, 0, len(messages))
	for _, message := range messages {
		if message.UID <= seenUpTo || message.IsRead {
			continue
		}
		arrived = append(arrived, message)
	}
	return arrived
}

func mailNotifyNewestUID(messages []mail.MessageResponse) uint32 {
	newest := uint32(0)
	for _, message := range messages {
		if message.UID > newest {
			newest = message.UID
		}
	}
	return newest
}

func mailNotifyExcerpt(subject string) string {
	trimmed := strings.TrimSpace(subject)
	runes := []rune(trimmed)
	if len(runes) <= mailNotifyLongest {
		return trimmed
	}
	return strings.TrimSpace(string(runes[:mailNotifyLongest])) + "…"
}
