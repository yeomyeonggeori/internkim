package admind

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
	"github.com/yeomyeonggeori/internkim/internal/buzzimport/relaypublish"
	"github.com/yeomyeonggeori/internkim/internal/centralplane"
)

const circleChannelSyncInterval = 2 * time.Minute

const circleChannelMemberRole = "member"

type buzzChannelLife int

const (
	buzzChannelAbsent buzzChannelLife = iota
	buzzChannelLive
	buzzChannelRetired
)

type circleSeatPlan struct {
	Seat   []string
	Unseat []string
}

type circleChannelKeeper struct {
	service     *Service
	record      *centralplane.Client
	relay       *sql.DB
	connections *buzzActorConnections
	agentSecret string
	agentPubkey string
}

func (service *Service) startCircleChannelSync(ctx context.Context) {
	if !service.canWriteToBuzzRelay() {
		return
	}
	go func() {
		ticker := time.NewTicker(circleChannelSyncInterval)
		defer ticker.Stop()
		for {
			service.withinASweepBudget(ctx, service.keepCircleChannelsToTheirCircles)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (service *Service) keepCircleChannelsToTheirCircles(ctx context.Context) {
	service.circleChannelKeeping.Lock()
	defer service.circleChannelKeeping.Unlock()
	keeper, isReady := service.circleChannelKeeper()
	if !isReady {
		return
	}
	defer keeper.connections.closeAll()
	circles, errorValue := keeper.record.CircleChannels(ctx)
	if errorValue != nil {
		log.Printf("circle channels: the record could not be read: %v", errorValue)
		return
	}
	for _, circle := range circles {
		if errorValue := keeper.keep(ctx, circle); errorValue != nil {
			log.Printf("circle channels: circle %s: %v", circle.CircleID, errorValue)
		}
	}
}

func (service *Service) circleChannelKeeper() (circleChannelKeeper, bool) {
	record := service.centralPlane()
	seed := service.buzzKeySeed()
	if record == nil || seed == "" || !service.canWriteToBuzzRelay() {
		return circleChannelKeeper{}, false
	}
	relay, errorValue := service.buzzDatabase()
	if errorValue != nil {
		log.Printf("circle channels: %v", errorValue)
		return circleChannelKeeper{}, false
	}
	agentSecret := buzzidentity.Secret(seed, buzzidentity.AgentSubject)
	agentPubkey, errorValue := buzzPublicKey(agentSecret)
	if errorValue != nil {
		log.Printf("circle channels: the agent has no key: %v", errorValue)
		return circleChannelKeeper{}, false
	}
	return circleChannelKeeper{
		service:     service,
		record:      record,
		relay:       relay,
		connections: service.newBuzzActorConnections(),
		agentSecret: agentSecret,
		agentPubkey: agentPubkey,
	}, true
}

func (keeper circleChannelKeeper) keep(ctx context.Context, circle centralplane.CircleChannel) error {
	if !circle.IsWanted() {
		return keeper.takeDown(ctx, circle)
	}
	channelID, errorValue := keeper.open(ctx, circle)
	if errorValue != nil {
		return errorValue
	}
	return keeper.seatExactly(ctx, channelID, circle.MemberEmails)
}

func (keeper circleChannelKeeper) open(ctx context.Context, circle centralplane.CircleChannel) (string, error) {
	channelID := circle.ChannelID
	life, errorValue := buzzChannelLifeOf(ctx, keeper.relay, channelID)
	if errorValue != nil {
		return "", errorValue
	}
	if life == buzzChannelLive {
		return channelID, nil
	}
	if channelID == "" || life == buzzChannelRetired {
		channelID = uuid.NewString()
		if errorValue := keeper.record.RecordCircleChannel(ctx, circle.CircleID, channelID); errorValue != nil {
			return "", errorValue
		}
	}
	errorValue = keeper.publish(ctx, func(publisher *relaypublish.Publisher) error {
		return publisher.CreateChannel(ctx, keeper.agentSecret, channelID, circle.ChannelName, "", "stream", "private")
	})
	if errorValue != nil {
		return "", fmt.Errorf("opening channel %s: %w", channelID, errorValue)
	}
	log.Printf("circle channels: circle %s opened channel %s", circle.CircleID, channelID)
	return channelID, nil
}

func (keeper circleChannelKeeper) takeDown(ctx context.Context, circle centralplane.CircleChannel) error {
	life, errorValue := buzzChannelLifeOf(ctx, keeper.relay, circle.ChannelID)
	if errorValue != nil {
		return errorValue
	}
	if life == buzzChannelLive {
		errorValue := keeper.publish(ctx, func(publisher *relaypublish.Publisher) error {
			return publisher.DeleteChannel(ctx, keeper.agentSecret, circle.ChannelID)
		})
		if errorValue != nil {
			return fmt.Errorf("deleting channel %s: %w", circle.ChannelID, errorValue)
		}
		log.Printf("circle channels: circle %s deleted channel %s", circle.CircleID, circle.ChannelID)
	}
	return keeper.record.RecordCircleChannel(ctx, circle.CircleID, "")
}

func (keeper circleChannelKeeper) seatExactly(ctx context.Context, channelID string, memberEmails []string) error {
	heldRoles, errorValue := buzzChannelMemberRoles(ctx, keeper.relay, channelID)
	if errorValue != nil {
		return errorValue
	}
	plan := planCircleSeats(heldRoles, keeper.pubkeysOf(ctx, memberEmails), keeper.agentPubkey)
	var failures []error
	for _, pubkey := range plan.Seat {
		failures = append(failures, keeper.publishPaced(ctx, func(publisher *relaypublish.Publisher) error {
			return publisher.AddMember(ctx, keeper.agentSecret, channelID, pubkey, circleChannelMemberRole)
		}))
	}
	for _, pubkey := range plan.Unseat {
		failures = append(failures, keeper.publishPaced(ctx, func(publisher *relaypublish.Publisher) error {
			return publisher.RemoveMember(ctx, keeper.agentSecret, channelID, pubkey)
		}))
	}
	return errors.Join(failures...)
}

func (keeper circleChannelKeeper) pubkeysOf(ctx context.Context, emails []string) map[string]bool {
	pubkeys := map[string]bool{}
	for _, email := range emails {
		secret := keeper.service.buzzSecretForEmail(ctx, email)
		if secret == "" {
			continue
		}
		pubkey, errorValue := buzzPublicKey(secret)
		if errorValue != nil {
			continue
		}
		pubkeys[pubkey] = true
	}
	return pubkeys
}

func (keeper circleChannelKeeper) publishPaced(ctx context.Context, act func(*relaypublish.Publisher) error) error {
	defer time.Sleep(buzzSeatPacing)
	return keeper.publish(ctx, act)
}

func (keeper circleChannelKeeper) publish(ctx context.Context, act func(*relaypublish.Publisher) error) error {
	publisher, errorValue := keeper.connections.as(ctx, keeper.agentSecret)
	if errorValue != nil {
		return errorValue
	}
	errorValue = act(publisher)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "connection closed") {
		return errorValue
	}
	keeper.connections.drop(keeper.agentSecret)
	publisher, errorValue = keeper.connections.as(ctx, keeper.agentSecret)
	if errorValue != nil {
		return errorValue
	}
	return act(publisher)
}

func planCircleSeats(heldRoles map[string]string, wanted map[string]bool, agentPubkey string) circleSeatPlan {
	plan := circleSeatPlan{Seat: []string{}, Unseat: []string{}}
	for pubkey := range wanted {
		if pubkey != agentPubkey && heldRoles[pubkey] != circleChannelMemberRole {
			plan.Seat = append(plan.Seat, pubkey)
		}
	}
	for pubkey := range heldRoles {
		if pubkey != agentPubkey && !wanted[pubkey] {
			plan.Unseat = append(plan.Unseat, pubkey)
		}
	}
	sort.Strings(plan.Seat)
	sort.Strings(plan.Unseat)
	return plan
}

func buzzChannelLifeOf(ctx context.Context, relay *sql.DB, channelID string) (buzzChannelLife, error) {
	if channelID == "" {
		return buzzChannelAbsent, nil
	}
	var isLive bool
	errorValue := relay.QueryRowContext(ctx,
		"SELECT deleted_at IS NULL FROM channels WHERE id = $1::uuid", channelID).Scan(&isLive)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return buzzChannelAbsent, nil
	}
	if errorValue != nil {
		return buzzChannelAbsent, errorValue
	}
	if isLive {
		return buzzChannelLive, nil
	}
	return buzzChannelRetired, nil
}
