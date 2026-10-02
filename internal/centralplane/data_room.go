package centralplane

import "context"

type DataRoomCategory struct {
	Code        string  `json:"code"`
	Parent      *string `json:"parent"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	ChoiceGroup *int    `json:"choiceGroup"`
}

func (client *Client) DataRoomCategories(ctx context.Context, requesterEmail string) ([]DataRoomCategory, error) {
	var answer struct {
		Categories []DataRoomCategory `json:"categories"`
	}
	if errorValue := client.runRecordTool(ctx, requesterEmail, "dataroom_get", map[string]string{}, &answer); errorValue != nil {
		return nil, errorValue
	}
	return answer.Categories, nil
}
