package admind

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type fakeAgentProfileMessenger struct {
	held           buzzProfileContent
	uploadedType   string
	uploads        int
	publishedName  string
	publishedImage string
	publishes      int
}

const fakeAgentPictureURL = "https://relay.example.com/sample-digest.png"

func (messenger *fakeAgentProfileMessenger) heldProfile(context.Context) (buzzProfileContent, error) {
	return messenger.held, nil
}

func (messenger *fakeAgentProfileMessenger) uploadPicture(_ context.Context, _ []byte, mimeType string) (string, error) {
	messenger.uploads++
	messenger.uploadedType = mimeType
	return fakeAgentPictureURL, nil
}

func (messenger *fakeAgentProfileMessenger) publishProfile(_ context.Context, displayName string, pictureURL string) error {
	messenger.publishes++
	messenger.publishedName = displayName
	messenger.publishedImage = pictureURL
	return nil
}

func shippedAgentPicture(t *testing.T) []byte {
	t.Helper()
	picture, errorValue := os.ReadFile(filepath.Join("..", "..", "assets", "internkim.square.png"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return picture
}

func TestAnAgentTheRelayHoldsNoProfileForIsPublishedByNameWithTheDefaultPicture(t *testing.T) {
	messenger := &fakeAgentProfileMessenger{}

	published, errorValue := giveTheAgentADefaultPicture(context.Background(), messenger, shippedAgentPicture(t))

	if errorValue != nil || !published {
		t.Fatalf("published %v, error %v; want the profile published", published, errorValue)
	}
	if messenger.publishedName != agentName || messenger.publishedImage != fakeAgentPictureURL {
		t.Fatalf("published %q with %q, want %q with the uploaded picture", messenger.publishedName, messenger.publishedImage, agentName)
	}
	if messenger.uploadedType != "image/png" {
		t.Fatalf("uploaded the picture as %q, want image/png", messenger.uploadedType)
	}
}

func TestAnAgentWithAPictureKeepsItAndNothingIsUploaded(t *testing.T) {
	messenger := &fakeAgentProfileMessenger{held: buzzProfileContent{Name: "Sample", Picture: "https://relay.example.com/chosen.png"}}

	published, errorValue := giveTheAgentADefaultPicture(context.Background(), messenger, shippedAgentPicture(t))

	if errorValue != nil || published {
		t.Fatalf("published %v, error %v; want nothing written", published, errorValue)
	}
	if messenger.uploads != 0 || messenger.publishes != 0 {
		t.Fatalf("uploaded %d and published %d times over a picture somebody set", messenger.uploads, messenger.publishes)
	}
}

func TestANamedAgentWithoutAPictureGainsOneUnderTheNameItHas(t *testing.T) {
	messenger := &fakeAgentProfileMessenger{held: buzzProfileContent{Name: "박예시", Display: "최견본"}}

	published, errorValue := giveTheAgentADefaultPicture(context.Background(), messenger, shippedAgentPicture(t))

	if errorValue != nil || !published {
		t.Fatalf("published %v, error %v; want the picture added", published, errorValue)
	}
	if messenger.publishedName != "최견본" || messenger.publishedImage != fakeAgentPictureURL {
		t.Fatalf("published %q with %q, want the name it already had with the uploaded picture", messenger.publishedName, messenger.publishedImage)
	}
}

func TestADefaultPictureThatIsNotAnImageIsRefusedBeforeAnythingIsUploaded(t *testing.T) {
	messenger := &fakeAgentProfileMessenger{}

	_, errorValue := giveTheAgentADefaultPicture(context.Background(), messenger, []byte("not an image"))

	if !errors.Is(errorValue, errAgentPictureIsNotAnImage) || messenger.uploads != 0 {
		t.Fatalf("error %v after %d uploads, want the picture refused before any upload", errorValue, messenger.uploads)
	}
}
