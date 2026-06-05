package admind

const (
	mailSecurityTLS      = "tls"
	mailSecurityStartTLS = "starttls"
	mailSecurityNone     = "none"
)

type mailAccount struct {
	ActorEmail     string
	Email          string
	FromAddress    string
	DisplayName    string
	IMAPHost       string
	IMAPPort       int
	IMAPSecurity   string
	IMAPUsername   string
	IMAPPassword   string
	SMTPHost       string
	SMTPPort       int
	SMTPSecurity   string
	SMTPUsername   string
	SMTPPassword   string
	DefaultMailbox string
	SentMailbox    string
	UpdatedAt      string
}

type mailAccountResponse struct {
	Email           string `json:"email"`
	FromAddress     string `json:"fromAddress"`
	DisplayName     string `json:"displayName"`
	IMAPHost        string `json:"imapHost"`
	IMAPPort        int    `json:"imapPort"`
	IMAPSecurity    string `json:"imapSecurity"`
	IMAPUsername    string `json:"imapUsername"`
	SMTPHost        string `json:"smtpHost"`
	SMTPPort        int    `json:"smtpPort"`
	SMTPSecurity    string `json:"smtpSecurity"`
	SMTPUsername    string `json:"smtpUsername"`
	DefaultMailbox  string `json:"defaultMailbox"`
	SentMailbox     string `json:"sentMailbox"`
	IsConfigured    bool   `json:"isConfigured"`
	HasIMAPPassword bool   `json:"hasIMAPPassword"`
	HasSMTPPassword bool   `json:"hasSMTPPassword"`
}

type mailAccountWriteRequest struct {
	Email          string `json:"email"`
	FromAddress    string `json:"fromAddress"`
	DisplayName    string `json:"displayName"`
	IMAPHost       string `json:"imapHost"`
	IMAPPort       int    `json:"imapPort"`
	IMAPSecurity   string `json:"imapSecurity"`
	IMAPUsername   string `json:"imapUsername"`
	IMAPPassword   string `json:"imapPassword"`
	SMTPHost       string `json:"smtpHost"`
	SMTPPort       int    `json:"smtpPort"`
	SMTPSecurity   string `json:"smtpSecurity"`
	SMTPUsername   string `json:"smtpUsername"`
	SMTPPassword   string `json:"smtpPassword"`
	DefaultMailbox string `json:"defaultMailbox"`
	SentMailbox    string `json:"sentMailbox"`
}
