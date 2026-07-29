package admind

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/buzzidentity"
	"gitlab.com/eastriver/internkim/internal/buzzimport/relaypublish"
)

func (service *Service) startStaffChannelMembershipSync(ctx context.Context) {
	if service.buzzKeySeed() == "" ||
		strings.TrimSpace(service.Configuration.BuzzRelayURL) == "" ||
		strings.TrimSpace(service.Configuration.BuzzDatabaseURL) == "" {
		return
	}
	go service.ensureStaffChannelMembership(ctx)
}

func (service *Service) ensureStaffChannelMembership(ctx context.Context) {
	seed := service.buzzKeySeed()
	if seed == "" {
		return
	}
	channelIDs, errorValue := service.buzzStreamChannelIDs(ctx)
	if errorValue != nil || len(channelIDs) == 0 {
		return
	}
	staffPubkeys := service.staffBuzzPubkeys(ctx)
	if len(staffPubkeys) == 0 {
		return
	}
	bootstrapSecret := buzzidentity.Secret(seed, buzzidentity.BootstrapSubject)
	publisher, errorValue := relaypublish.Connect(ctx, strings.TrimSpace(service.Configuration.BuzzRelayURL), bootstrapSecret)
	if errorValue != nil {
		return
	}
	defer publisher.Close()
	for _, channelID := range channelIDs {
		for _, pubkey := range staffPubkeys {
			select {
			case <-ctx.Done():
				return
			default:
			}
			_ = publisher.AddMember(ctx, bootstrapSecret, channelID, pubkey)
			time.Sleep(60 * time.Millisecond)
		}
	}
}

func (service *Service) buzzStreamChannelIDs(ctx context.Context) ([]string, error) {
	database, errorValue := sql.Open("postgres", strings.TrimSpace(service.Configuration.BuzzDatabaseURL))
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, "SELECT id FROM channels WHERE channel_type = 'stream'")
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	var channelIDs []string
	for rows.Next() {
		var id string
		if errorValue := rows.Scan(&id); errorValue != nil {
			continue
		}
		channelIDs = append(channelIDs, id)
	}
	return channelIDs, rows.Err()
}

func (service *Service) staffBuzzPubkeys(ctx context.Context) []string {
	seen := map[string]bool{}
	var pubkeys []string
	for _, record := range service.blueclawPolicyUserRecords(ctx) {
		email := strings.ToLower(strings.TrimSpace(record.Email))
		if email == "" {
			continue
		}
		secretHex := service.buzzSecretForEmail(ctx, email)
		if secretHex == "" {
			continue
		}
		pubkey, errorValue := buzzPublicKey(secretHex)
		if errorValue != nil || seen[pubkey] {
			continue
		}
		seen[pubkey] = true
		pubkeys = append(pubkeys, pubkey)
	}
	return pubkeys
}
