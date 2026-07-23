package capabilityd

import (
	"context"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestDecodePlatformMessageSendInputFlatChannel(t *testing.T) {
	input, errorValue := decodePlatformMessageSendInput([]byte(`{"targetType":"channel","channelName":"광장","message":"공지"}`))
	if errorValue != nil {
		t.Fatalf("expected flat channel send to decode, got %v", errorValue)
	}
	if input.DeliveryTarget.Type != "channel" || input.DeliveryTarget.ChannelName != "광장" {
		t.Fatalf("expected channel target 광장, got %+v", input.DeliveryTarget)
	}
}

func TestDecodePlatformMessageSendInputRejectsMissingTargetType(t *testing.T) {
	_, errorValue := decodePlatformMessageSendInput([]byte(`{"channelName":"광장","message":"공지"}`))
	if errorValue == nil {
		t.Fatal("expected missing targetType to be rejected")
	}
	if errorValue.Error() != "targetType must be directMessage, currentThread, currentChannel, or channel" {
		t.Fatalf("expected targetType guidance error, got %q", errorValue.Error())
	}
}

func TestDecodePlatformMessageSendInputDirectMessageDefaultsToRequester(t *testing.T) {
	input, errorValue := decodePlatformMessageSendInput([]byte(`{"targetType":"directMessage","message":"확인 부탁드립니다"}`))
	if errorValue != nil {
		t.Fatalf("expected a hint-less direct message to decode as a self send, got %v", errorValue)
	}
	if input.DeliveryTarget.Type != "directMessage" || input.DeliveryTarget.PersonHint != "" {
		t.Fatalf("expected an empty personHint to survive decoding, got %+v", input.DeliveryTarget)
	}
}

func TestResolvePlatformDirectSendRecipientUsesRequesterWithoutHint(t *testing.T) {
	service := Service{}
	request := capabilities.ToolInvokeRequest{Context: capabilities.ToolInvokeContext{RequesterPlatformUserID: "requester-user-1"}}

	recipientUserID, _, hasFailure := service.resolvePlatformDirectSendRecipient(context.Background(), request, "")

	if hasFailure || recipientUserID != "requester-user-1" {
		t.Fatalf("expected the requester to receive a hint-less direct message, got %q failure=%v", recipientUserID, hasFailure)
	}
}

func TestResolvePlatformDirectSendRecipientFailsClosedWithoutRequesterIdentity(t *testing.T) {
	service := Service{}
	request := capabilities.ToolInvokeRequest{}

	_, failure, hasFailure := service.resolvePlatformDirectSendRecipient(context.Background(), request, "")

	if !hasFailure || failure.ErrorCode != "invalid_input" {
		t.Fatalf("expected a loud failure without requester identity, got failure=%+v hasFailure=%v", failure, hasFailure)
	}
}

func TestDecodePlatformMessageSendInputChannelAcceptsBothIdentifiers(t *testing.T) {
	input, errorValue := decodePlatformMessageSendInput([]byte(`{"targetType":"channel","channelID":"c-1","channelName":"광장","message":"공지"}`))
	if errorValue != nil {
		t.Fatalf("expected both channel identifiers to be accepted, got %v", errorValue)
	}
	if input.DeliveryTarget.ChannelID != "c-1" {
		t.Fatalf("expected channelID retained for precedence, got %+v", input.DeliveryTarget)
	}
}
