package admind

import (
	"testing"

	"gitlab.com/eastriver/internkim/internal/mail"
)

func TestOnlyUnreadMailAboveTheMarkIsNews(t *testing.T) {
	messages := []mail.MessageResponse{
		{UID: 12, Subject: "새 메일", IsRead: false},
		{UID: 11, Subject: "이미 읽음", IsRead: true},
		{UID: 9, Subject: "예전 것", IsRead: false},
	}

	arrived := mailNotifyArrivedSince(messages, 10)
	if len(arrived) != 1 || arrived[0].UID != 12 {
		t.Fatalf("arrived = %+v", arrived)
	}
}

func TestTheMarkMovesPastWhatWasSeenReadOrNot(t *testing.T) {
	messages := []mail.MessageResponse{
		{UID: 12, IsRead: true},
		{UID: 15, IsRead: false},
		{UID: 3, IsRead: false},
	}
	if newest := mailNotifyNewestUID(messages); newest != 15 {
		t.Fatalf("newest = %d", newest)
	}
	if newest := mailNotifyNewestUID(nil); newest != 0 {
		t.Fatalf("an empty inbox marks nothing, got %d", newest)
	}
}

func TestOneSenderIsNamedAndTheRestAreCounted(t *testing.T) {
	single := mailNotifyNotification([]mail.MessageResponse{{From: "이샘플", Subject: "안건"}}, "mm-1")
	if single.Title != "이샘플" || single.Body != "안건" {
		t.Fatalf("single = %+v", single)
	}
	if single.Category != "mail" || single.OpenPath != "/mail/" {
		t.Fatalf("single = %+v", single)
	}

	many := mailNotifyNotification([]mail.MessageResponse{
		{From: "이샘플", Subject: "안건"},
		{From: "박예시"},
		{From: "최견본"},
	}, "mm-1")
	if many.Title != "이샘플 외 2명" {
		t.Fatalf("title = %q", many.Title)
	}
}

func TestAMailWithNoSenderStillSaysSomething(t *testing.T) {
	notification := mailNotifyNotification([]mail.MessageResponse{{Subject: "제목만 있음"}}, "mm-1")
	if notification.Title != "메일" {
		t.Fatalf("title = %q", notification.Title)
	}
}

func TestOnlySomebodyWithABothAddressAndAnAccountIsMailed(t *testing.T) {
	externalIDs := mailNotifyExternalIDs([]adminUserMutation{
		{Email: "Member1@Example.com", MattermostUserID: "mm-1"},
		{Email: "", MattermostUserID: "mm-2"},
		{Email: "member3@example.com", MattermostUserID: ""},
	})
	if len(externalIDs) != 1 || externalIDs["member1@example.com"] != "mm-1" {
		t.Fatalf("externalIDs = %+v", externalIDs)
	}
}
