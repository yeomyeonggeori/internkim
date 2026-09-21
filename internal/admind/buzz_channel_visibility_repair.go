package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
)

type buzzChannelVisibilityCoverage struct {
	Bridged      int      `json:"bridged"`
	OpenInBuzz   int      `json:"openInBuzz"`
	ShouldClose  int      `json:"shouldClose"`
	Closed       int      `json:"closed"`
	ClosingNames []string `json:"closingNames"`
}

func (service *Service) handleBuzzChannelVisibilityRepair(responseWriter http.ResponseWriter, request *http.Request) {
	shouldClose := request.URL.Query().Get("close") == "true"
	coverage, errorValue := service.buzzChannelsOpenerThanTheRoomTheyMirror(request.Context(), shouldClose)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(coverage)
}

func (service *Service) buzzChannelsOpenerThanTheRoomTheyMirror(ctx context.Context, shouldClose bool) (buzzChannelVisibilityCoverage, error) {
	mappings, errorValue := service.bridgedMattermostChannels(ctx)
	if errorValue != nil {
		return buzzChannelVisibilityCoverage{}, errorValue
	}
	relay, errorValue := service.buzzDatabase()
	if errorValue != nil {
		return buzzChannelVisibilityCoverage{}, errorValue
	}

	coverage := buzzChannelVisibilityCoverage{Bridged: len(mappings), ClosingNames: []string{}}
	for _, mapping := range mappings {
		if errorValue := service.countAndCloseOneChannel(ctx, relay, mapping, shouldClose, &coverage); errorValue != nil {
			return coverage, errorValue
		}
	}
	return coverage, nil
}

func (service *Service) countAndCloseOneChannel(
	ctx context.Context,
	relay *sql.DB,
	mapping bridgeChannelMapping,
	shouldClose bool,
	coverage *buzzChannelVisibilityCoverage,
) error {
	isOpen, errorValue := buzzChannelIsOpen(ctx, relay, mapping.BuzzChannelID)
	if errorValue != nil || !isOpen {
		return errorValue
	}
	coverage.OpenInBuzz++
	shape, errorValue := service.describeBridgeRelayChannel(ctx, "mattermost", mapping.ExternalChannelID)
	if errorValue != nil || shape.Visibility == "open" {
		return errorValue
	}
	coverage.ShouldClose++
	coverage.ClosingNames = append(coverage.ClosingNames, shape.Name)
	if !shouldClose {
		return nil
	}
	if errorValue := closeBuzzChannel(ctx, relay, mapping.BuzzChannelID); errorValue != nil {
		return errorValue
	}
	coverage.Closed++
	log.Printf("buzz channel closed to match the room it mirrors: %s", shape.Name)
	return nil
}

func (service *Service) bridgedMattermostChannels(ctx context.Context) ([]bridgeChannelMapping, error) {
	database, errorValue := service.openBridgeMapDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	return service.listBridgeChannels(ctx, database, "mattermost")
}

func buzzChannelIsOpen(ctx context.Context, relay *sql.DB, buzzChannelID string) (bool, error) {
	var visibility string
	errorValue := relay.QueryRowContext(ctx,
		"SELECT visibility FROM channels WHERE id = $1 AND deleted_at IS NULL", buzzChannelID,
	).Scan(&visibility)
	if errorValue == sql.ErrNoRows {
		return false, nil
	}
	return visibility == "open", errorValue
}

func closeBuzzChannel(ctx context.Context, relay *sql.DB, buzzChannelID string) error {
	_, errorValue := relay.ExecContext(ctx,
		"UPDATE channels SET visibility = 'private' WHERE id = $1", buzzChannelID)
	return errorValue
}
