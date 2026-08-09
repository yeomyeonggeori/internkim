package mail

type MessageListResponse struct {
	Messages    []MessageResponse `json:"messages"`
	NextCursor  string            `json:"nextCursor"`
	UIDNext     uint32            `json:"uidNext,omitempty"`
	UIDValidity uint32            `json:"uidValidity,omitempty"`
}

type MessageResponse struct {
	UID     uint32 `json:"uid"`
	Mailbox string `json:"mailbox"`
	Subject string `json:"subject"`
	From    string `json:"from"`
	Date    string `json:"date"`
	Preview string `json:"preview"`
	IsRead  bool   `json:"isRead"`
}

type MessageDetailResponse struct {
	UID      uint32 `json:"uid"`
	Mailbox  string `json:"mailbox"`
	Subject  string `json:"subject"`
	From     string `json:"from"`
	To       string `json:"to"`
	CC       string `json:"cc"`
	Date     string `json:"date"`
	Body     string `json:"body"`
	BodyHTML string `json:"bodyHTML,omitempty"`
	IsRead   bool   `json:"isRead"`
}

type SendResult struct {
	Sent          bool   `json:"sent"`
	AppendedTo    string `json:"appendedTo,omitempty"`
	AppendWarning string `json:"appendWarning,omitempty"`
}
