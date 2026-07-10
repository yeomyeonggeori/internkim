package capabilityd

import "testing"

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

func TestDecodePlatformMessageSendInputChannelAcceptsBothIdentifiers(t *testing.T) {
	input, errorValue := decodePlatformMessageSendInput([]byte(`{"targetType":"channel","channelID":"c-1","channelName":"광장","message":"공지"}`))
	if errorValue != nil {
		t.Fatalf("expected both channel identifiers to be accepted, got %v", errorValue)
	}
	if input.DeliveryTarget.ChannelID != "c-1" {
		t.Fatalf("expected channelID retained for precedence, got %+v", input.DeliveryTarget)
	}
}
