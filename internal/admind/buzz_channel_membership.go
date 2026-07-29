package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
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
	if errorValue != nil {
		log.Printf("buzz staff membership: channel query failed: %v", errorValue)
		return
	}
	staffPubkeys := service.staffBuzzPubkeys(ctx)
	log.Printf("buzz staff membership: %d stream channels, %d staff pubkeys", len(channelIDs), len(staffPubkeys))
	if len(channelIDs) == 0 || len(staffPubkeys) == 0 {
		return
	}
	bootstrapSecret := buzzidentity.Secret(seed, buzzidentity.BootstrapSubject)
	publisher, errorValue := relaypublish.Connect(ctx, strings.TrimSpace(service.Configuration.BuzzRelayURL), bootstrapSecret)
	if errorValue != nil {
		log.Printf("buzz staff membership: relay connect failed: %v", errorValue)
		return
	}
	defer publisher.Close()
	granted, failed := 0, 0
	for _, channelID := range channelIDs {
		for _, pubkey := range staffPubkeys {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if errorValue := publisher.AddMember(ctx, bootstrapSecret, channelID, pubkey); errorValue != nil {
				if failed == 0 {
					log.Printf("buzz staff membership: first AddMember error: %v", errorValue)
				}
				failed++
			} else {
				granted++
			}
			time.Sleep(60 * time.Millisecond)
		}
	}
	log.Printf("buzz staff membership: granted %d, failed %d", granted, failed)
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

func (service *Service) allStaffEmails(ctx context.Context) []string {
	seen := map[string]bool{}
	var emails []string
	add := func(email string) {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" || seen[email] {
			return
		}
		seen[email] = true
		emails = append(emails, email)
	}
	for _, record := range service.blueclawPolicyUserRecords(ctx) {
		add(record.Email)
	}
	fleetID := strings.ToLower(strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)))
	fleetSecret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	if fleetID != "" && fleetSecret != "" {
		if records, errorValue := service.lookupUserRecords(ctx, fleetID, fleetSecret); errorValue == nil {
			for _, record := range records {
				add(record.Email)
			}
		}
	}
	for _, email := range service.usersSyncCacheEmails() {
		add(email)
	}
	add(service.seedAdminEmail())
	return emails
}

func (service *Service) usersSyncCacheEmails() []string {
	stateDirectory := filepath.Dir(service.Configuration.FlowDatabasePath)
	content, errorValue := os.ReadFile(filepath.Join(stateDirectory, "users-sync.json"))
	if errorValue != nil {
		return nil
	}
	var cache struct {
		Users []string `json:"users"`
	}
	if json.Unmarshal(content, &cache) != nil {
		return nil
	}
	return cache.Users
}

func (service *Service) staffBuzzPubkeys(ctx context.Context) []string {
	seen := map[string]bool{}
	var pubkeys []string
	for _, email := range service.allStaffEmails(ctx) {
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
