package admind

type crmHTTPAudit struct {
	CreatedAt          string `json:"createdAt"`
	CreatedByPersonID  string `json:"createdByPersonID"`
	UpdatedAt          string `json:"updatedAt"`
	UpdatedByPersonID  string `json:"updatedByPersonID"`
	ArchivedAt         string `json:"archivedAt,omitempty"`
	ArchivedByPersonID string `json:"archivedByPersonID,omitempty"`
}

type crmHTTPAccount struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Status        string       `json:"status"`
	Types         []string     `json:"types"`
	Tags          []string     `json:"tags"`
	Importance    string       `json:"importance"`
	OwnerPersonID string       `json:"ownerPersonID"`
	OwnerCircleID string       `json:"ownerCircleID,omitempty"`
	Address       string       `json:"address,omitempty"`
	Description   string       `json:"description,omitempty"`
	Audit         crmHTTPAudit `json:"audit"`
}

type crmHTTPContact struct {
	ID            string       `json:"id"`
	AccountID     string       `json:"accountID"`
	Name          string       `json:"name"`
	Email         string       `json:"email,omitempty"`
	Phone         string       `json:"phone,omitempty"`
	Title         string       `json:"title,omitempty"`
	Department    string       `json:"department,omitempty"`
	IsPrimary     bool         `json:"isPrimary"`
	OwnerPersonID string       `json:"ownerPersonID"`
	OwnerCircleID string       `json:"ownerCircleID,omitempty"`
	Description   string       `json:"description,omitempty"`
	Audit         crmHTTPAudit `json:"audit"`
}

type crmHTTPOpportunityContact struct {
	ContactID string `json:"contactID"`
	IsPrimary bool   `json:"isPrimary"`
}

type crmHTTPOpportunity struct {
	ID               string                      `json:"id"`
	AccountID        string                      `json:"accountID,omitempty"`
	Business         string                      `json:"business,omitempty"`
	Name             string                      `json:"name"`
	Pipeline         string                      `json:"pipeline"`
	Stage            string                      `json:"stage"`
	StagePosition    float64                     `json:"stagePosition"`
	StageChangedAt   string                      `json:"stageChangedAt"`
	OwnerPersonID    string                      `json:"ownerPersonID"`
	OwnerCircleID    string                      `json:"ownerCircleID,omitempty"`
	AmountMinor      *int64                      `json:"amountMinor,omitempty"`
	CurrencyCode     string                      `json:"currencyCode"`
	BaseAmountMinor  *int64                      `json:"baseAmountMinor,omitempty"`
	BaseCurrencyCode string                      `json:"baseCurrencyCode,omitempty"`
	Importance       string                      `json:"importance"`
	DueAt            string                      `json:"dueAt,omitempty"`
	DueTimeZone      string                      `json:"dueTimeZone,omitempty"`
	LostReason       string                      `json:"lostReason,omitempty"`
	Description      string                      `json:"description,omitempty"`
	Contacts         []crmHTTPOpportunityContact `json:"contacts,omitempty"`
	Audit            crmHTTPAudit                `json:"audit"`
}

type crmHTTPActivity struct {
	ID            string       `json:"id"`
	AccountID     string       `json:"accountID,omitempty"`
	ContactID     string       `json:"contactID,omitempty"`
	OpportunityID string       `json:"opportunityID,omitempty"`
	Business      string       `json:"business,omitempty"`
	Kind          string       `json:"kind"`
	Title         string       `json:"title"`
	OccurredAt    string       `json:"occurredAt"`
	Content       string       `json:"content,omitempty"`
	Audit         crmHTTPAudit `json:"audit"`
}

type crmHTTPAccountPayload struct {
	Name          string   `json:"name"`
	Status        string   `json:"status"`
	Types         []string `json:"types"`
	Tags          []string `json:"tags"`
	Importance    string   `json:"importance"`
	OwnerPersonID string   `json:"ownerPersonID"`
	OwnerCircleID string   `json:"ownerCircleID"`
	Address       string   `json:"address"`
	Description   string   `json:"description"`
}

type crmHTTPContactPayload struct {
	AccountID     string `json:"accountID"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	Title         string `json:"title"`
	Department    string `json:"department"`
	IsPrimary     bool   `json:"isPrimary"`
	OwnerPersonID string `json:"ownerPersonID"`
	OwnerCircleID string `json:"ownerCircleID"`
	Description   string `json:"description"`
}

type crmHTTPOpportunityPayload struct {
	AccountID     string                      `json:"accountID"`
	Business      string                      `json:"business"`
	Name          string                      `json:"name"`
	Pipeline      string                      `json:"pipeline"`
	OwnerPersonID string                      `json:"ownerPersonID"`
	OwnerCircleID string                      `json:"ownerCircleID"`
	AmountMinor   *int64                      `json:"amountMinor"`
	CurrencyCode  string                      `json:"currencyCode"`
	Importance    string                      `json:"importance"`
	DueAt         string                      `json:"dueAt"`
	DueTimeZone   string                      `json:"dueTimeZone"`
	Description   string                      `json:"description"`
	Contacts      []crmHTTPOpportunityContact `json:"contacts"`
	Transition    *crmHTTPTransitionPayload   `json:"transition,omitempty"`
}

type crmHTTPActivityPayload struct {
	AccountID     string `json:"accountID"`
	ContactID     string `json:"contactID"`
	OpportunityID string `json:"opportunityID"`
	Business      string `json:"business"`
	Kind          string `json:"kind"`
	Title         string `json:"title"`
	OccurredAt    string `json:"occurredAt"`
	Content       string `json:"content"`
}

type crmHTTPTransitionPayload struct {
	Stage               string  `json:"stage"`
	StagePosition       float64 `json:"stagePosition"`
	BeforeOpportunityID string  `json:"beforeOpportunityID"`
	OccurredAt          string  `json:"occurredAt"`
	LostReason          string  `json:"lostReason"`
	BaseAmountMinor     *int64  `json:"baseAmountMinor"`
	BaseCurrencyCode    string  `json:"baseCurrencyCode"`
}

type crmHTTPPositionPayload struct {
	Position            float64 `json:"position"`
	BeforeOpportunityID string  `json:"beforeOpportunityID"`
	UpdatedAt           string  `json:"updatedAt"`
}

type crmHTTPPipeline struct {
	Pipeline  string `json:"pipeline"`
	Label     string `json:"label"`
	Direction string `json:"direction"`
	IsActive  bool   `json:"isActive"`
}

type crmHTTPLostReason struct {
	Reason   string `json:"reason"`
	Label    string `json:"label"`
	IsActive bool   `json:"isActive"`
}

type crmHTTPErrorDocument struct {
	Error crmHTTPError `json:"error"`
}

type crmHTTPError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
