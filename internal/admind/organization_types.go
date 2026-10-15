package admind

type orgGroupRecord struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ParentID string `json:"parentID,omitempty"`
}
