package centralplane

import "context"

type CompanyMetric struct {
	MetricID  string   `json:"metricID"`
	Metric    string   `json:"metric"`
	Year      int      `json:"year"`
	Quarter   int      `json:"quarter"`
	Month     int      `json:"month"`
	Value     float64  `json:"value"`
	Currency  string   `json:"currency"`
	ValueUSD  *float64 `json:"valueUSD"`
	Unit      string   `json:"unit"`
	Note      string   `json:"note"`
	UpdatedAt string   `json:"updatedAt"`
}

type CompanyRecordAttribute struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type CompanyRecord struct {
	RecordID   string                   `json:"recordID"`
	Category   string                   `json:"category"`
	Date       string                   `json:"date"`
	Title      string                   `json:"title"`
	Detail     string                   `json:"detail"`
	Attributes []CompanyRecordAttribute `json:"attributes"`
	UpdatedAt  string                   `json:"updatedAt"`
}

type CompanyDocumentPublished struct {
	At   string `json:"at"`
	By   string `json:"by"`
	From string `json:"from"`
}

type CompanyDocument struct {
	DocumentID     string                    `json:"documentID"`
	DocumentNumber string                    `json:"documentNumber"`
	Kind           string                    `json:"kind"`
	DocumentType   string                    `json:"documentType"`
	Title          string                    `json:"title"`
	Counterpart    string                    `json:"counterpart"`
	Language       string                    `json:"language"`
	FilePath       string                    `json:"filePath"`
	Summary        string                    `json:"summary"`
	RequesterID    string                    `json:"requesterID"`
	IssuedAt       string                    `json:"issuedAt"`
	CategoryCode   string                    `json:"categoryCode"`
	Date           string                    `json:"date"`
	Period         string                    `json:"period"`
	Status         string                    `json:"status"`
	Supersedes     string                    `json:"supersedes"`
	SHA256         string                    `json:"sha256"`
	Tags           []string                  `json:"tags"`
	StoragePath    string                    `json:"storagePath"`
	Published      *CompanyDocumentPublished `json:"published"`
}

func (client *Client) CompanyMetrics(ctx context.Context, requesterEmail string) ([]CompanyMetric, error) {
	var answered struct {
		Metrics []CompanyMetric `json:"metrics"`
	}
	if errorValue := client.runRecordTool(ctx, requesterEmail, "company_metric_list", map[string]any{}, &answered); errorValue != nil {
		return nil, errorValue
	}
	return answered.Metrics, nil
}

func (client *Client) CompanyRecords(ctx context.Context, requesterEmail string) ([]CompanyRecord, error) {
	var answered struct {
		Records []CompanyRecord `json:"records"`
	}
	if errorValue := client.runRecordTool(ctx, requesterEmail, "company_record_list", map[string]any{}, &answered); errorValue != nil {
		return nil, errorValue
	}
	return answered.Records, nil
}

func (client *Client) CompanyDocuments(ctx context.Context, requesterEmail string) ([]CompanyDocument, error) {
	var answered struct {
		Documents []CompanyDocument `json:"documents"`
	}
	if errorValue := client.runRecordTool(ctx, requesterEmail, "company_document_list", map[string]any{}, &answered); errorValue != nil {
		return nil, errorValue
	}
	return answered.Documents, nil
}
