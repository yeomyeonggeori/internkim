package admind

import (
	"sort"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

func messageListUIDs(client *imapclient.Client, selectedMailbox *imap.SelectData, input mailMessageListRequest) ([]imap.UID, error) {
	if selectedMailbox == nil || selectedMailbox.NumMessages == 0 {
		return nil, nil
	}
	if input.BeforeUID == 1 {
		return nil, nil
	}
	criteria := mailMessageSearchCriteria(selectedMailbox, input)
	searchData, errorValue := client.UIDSearch(criteria, nil).Wait()
	if errorValue != nil {
		return nil, errorValue
	}
	return mailMessagePageUIDs(searchData.AllUIDs(), input.Limit+1), nil
}

func mailMessageSearchCriteria(selectedMailbox *imap.SelectData, input mailMessageListRequest) *imap.SearchCriteria {
	criteria := &imap.SearchCriteria{}
	if strings.TrimSpace(input.Query) != "" {
		criteria.Text = []string{strings.TrimSpace(input.Query)}
	}
	maximumUID := uint32(selectedMailbox.UIDNext)
	if maximumUID > 0 {
		maximumUID--
	}
	if input.BeforeUID > 0 && (maximumUID == 0 || input.BeforeUID <= maximumUID) {
		maximumUID = input.BeforeUID - 1
	}
	if maximumUID > 0 {
		uidSet := imap.UIDSet{}
		uidSet.AddRange(imap.UID(1), imap.UID(maximumUID))
		criteria.UID = []imap.UIDSet{uidSet}
	}
	return criteria
}

func mailMessagePageUIDs(uids []imap.UID, limit int) []imap.UID {
	if len(uids) == 0 || limit <= 0 {
		return nil
	}
	sort.Slice(uids, func(firstIndex int, secondIndex int) bool {
		return uids[firstIndex] > uids[secondIndex]
	})
	if len(uids) > limit {
		return uids[:limit]
	}
	return uids
}

func visibleMailMessageUIDs(uids []imap.UID, limit int) ([]imap.UID, bool) {
	if len(uids) <= limit {
		return uids, false
	}
	return uids[:limit], true
}

func nextMailMessageCursor(input mailMessageListRequest, uids []imap.UID, hasMoreMessages bool) string {
	if !hasMoreMessages || len(uids) == 0 || input.Limit <= 0 {
		return ""
	}
	oldestUID := uids[len(uids)-1]
	if oldestUID <= 1 {
		return ""
	}
	return encodeMailMessageCursor(input.Mailbox, input.Query, uint32(oldestUID))
}

func containsMailFlag(flags []imap.Flag, flag imap.Flag) bool {
	for _, value := range flags {
		if value == flag {
			return true
		}
	}
	return false
}

func containsMailMailboxAttribute(attributes []imap.MailboxAttr, attribute imap.MailboxAttr) bool {
	for _, value := range attributes {
		if value == attribute {
			return true
		}
	}
	return false
}

func storeMailFlag(client *imapclient.Client, uid uint32, flag imap.Flag, enabled bool) error {
	operation := imap.StoreFlagsDel
	if enabled {
		operation = imap.StoreFlagsAdd
	}
	return client.Store(imap.UIDSetNum(imap.UID(uid)), &imap.StoreFlags{
		Op:     operation,
		Silent: true,
		Flags:  []imap.Flag{flag},
	}, nil).Close()
}
