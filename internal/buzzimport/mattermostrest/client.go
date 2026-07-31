package mattermostrest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"gitlab.com/eastriver/internkim/internal/buzzimport"
)

type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

func (client Client) httpClient() *http.Client {
	if client.HTTPClient != nil {
		return client.HTTPClient
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (client Client) get(ctx context.Context, path string, target any) error {
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, client.BaseURL+path, nil)
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Authorization", "Bearer "+client.Token)
	response, errorValue := client.httpClient().Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("mattermost %s returned %d", path, response.StatusCode)
	}
	return json.NewDecoder(response.Body).Decode(target)
}

type restUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Nickname string `json:"nickname"`
	IsBot    bool   `json:"is_bot"`
}

type restChannel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Purpose     string `json:"purpose"`
	Type        string `json:"type"`
}

type restPost struct {
	ID           string   `json:"id"`
	ChannelID    string   `json:"channel_id"`
	UserID       string   `json:"user_id"`
	RootID       string   `json:"root_id"`
	Message      string   `json:"message"`
	CreateAt     int64    `json:"create_at"`
	DeleteAt     int64    `json:"delete_at"`
	Type         string   `json:"type"`
	FileIDs      []string `json:"file_ids"`
	HasReactions bool     `json:"has_reactions"`
}

type Reaction struct {
	UserID    string `json:"user_id"`
	EmojiName string `json:"emoji_name"`
	CreateAt  int64  `json:"create_at"`
}

type restPostPage struct {
	Order []string            `json:"order"`
	Posts map[string]restPost `json:"posts"`
}

func (client Client) Users(ctx context.Context) ([]restUser, error) {
	users := []restUser{}
	for page := 0; ; page++ {
		var pageUsers []restUser
		if errorValue := client.get(ctx, "/api/v4/users?per_page=200&page="+strconv.Itoa(page), &pageUsers); errorValue != nil {
			return nil, errorValue
		}
		if len(pageUsers) == 0 {
			break
		}
		users = append(users, pageUsers...)
	}
	return users, nil
}

func (client Client) PublicChannels(ctx context.Context, teamID string) ([]restChannel, error) {
	channels := []restChannel{}
	for page := 0; ; page++ {
		var pageChannels []restChannel
		path := "/api/v4/teams/" + url.PathEscape(teamID) + "/channels?per_page=200&page=" + strconv.Itoa(page)
		if errorValue := client.get(ctx, path, &pageChannels); errorValue != nil {
			return nil, errorValue
		}
		if len(pageChannels) == 0 {
			break
		}
		channels = append(channels, pageChannels...)
	}
	return channels, nil
}

// Posts pulls a channel's whole history oldest first, paging backfrom the newest
// page, which is the order an import needs so a reply resolves its root.
func (client Client) Posts(ctx context.Context, channelID string, sinceMillis int64) ([]buzzimport.MattermostPost, error) {
	collected := map[string]restPost{}
	order := []string{}
	beforePostID := ""
	for {
		path := "/api/v4/channels/" + url.PathEscape(channelID) + "/posts?per_page=200"
		if beforePostID != "" {
			path += "&before=" + url.QueryEscape(beforePostID)
		}
		var page restPostPage
		if errorValue := client.get(ctx, path, &page); errorValue != nil {
			return nil, errorValue
		}
		if len(page.Order) == 0 {
			break
		}
		for _, postID := range page.Order {
			if _, seen := collected[postID]; !seen {
				collected[postID] = page.Posts[postID]
				order = append(order, postID)
			}
		}
		oldestInPage := page.Posts[page.Order[len(page.Order)-1]]
		if sinceMillis > 0 && oldestInPage.CreateAt <= sinceMillis {
			break
		}
		beforePostID = page.Order[len(page.Order)-1]
	}
	posts := make([]buzzimport.MattermostPost, 0, len(order))
	for _, postID := range order {
		post := collected[postID]
		if post.DeleteAt != 0 || post.Type != "" || (post.Message == "" && len(post.FileIDs) == 0) {
			continue
		}
		if sinceMillis > 0 && post.CreateAt <= sinceMillis {
			continue
		}
		posts = append(posts, buzzimport.MattermostPost{
			ID:        post.ID,
			ChannelID: post.ChannelID,
			UserID:    post.UserID,
			RootID:    post.RootID,
			Message:   post.Message,
			CreatedAt:    time.UnixMilli(post.CreateAt).UTC(),
			FileIDs:      post.FileIDs,
			HasReactions: post.HasReactions,
		})
	}
	sortPostsByTime(posts)
	return posts, nil
}

func (client Client) getBytes(ctx context.Context, path string) ([]byte, string, error) {
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, client.BaseURL+path, nil)
	if errorValue != nil {
		return nil, "", errorValue
	}
	request.Header.Set("Authorization", "Bearer "+client.Token)
	response, errorValue := client.httpClient().Do(request)
	if errorValue != nil {
		return nil, "", errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, "", fmt.Errorf("mattermost %s returned %d", path, response.StatusCode)
	}
	content, errorValue := io.ReadAll(response.Body)
	if errorValue != nil {
		return nil, "", errorValue
	}
	return content, response.Header.Get("Content-Type"), nil
}

func (client Client) Reactions(ctx context.Context, postID string) ([]Reaction, error) {
	var reactions []Reaction
	if errorValue := client.get(ctx, "/api/v4/posts/"+url.PathEscape(postID)+"/reactions", &reactions); errorValue != nil {
		return nil, errorValue
	}
	return reactions, nil
}

func (client Client) ChannelMemberUserIDs(ctx context.Context, channelID string) ([]string, error) {
	userIDs := []string{}
	for page := 0; ; page++ {
		var members []struct {
			UserID string `json:"user_id"`
		}
		path := "/api/v4/channels/" + url.PathEscape(channelID) + "/members?per_page=200&page=" + strconv.Itoa(page)
		if errorValue := client.get(ctx, path, &members); errorValue != nil {
			return nil, errorValue
		}
		if len(members) == 0 {
			break
		}
		for _, member := range members {
			userIDs = append(userIDs, member.UserID)
		}
	}
	return userIDs, nil
}

func (client Client) FileBytes(ctx context.Context, fileID string) ([]byte, string, error) {
	return client.getBytes(ctx, "/api/v4/files/"+url.PathEscape(fileID))
}

func (client Client) EmojiImage(ctx context.Context, emojiName string) ([]byte, string, error) {
	var emojiRecord struct {
		ID string `json:"id"`
	}
	if errorValue := client.get(ctx, "/api/v4/emoji/name/"+url.PathEscape(emojiName), &emojiRecord); errorValue != nil {
		return nil, "", errorValue
	}
	if emojiRecord.ID == "" {
		return nil, "", fmt.Errorf("custom emoji %q not found", emojiName)
	}
	return client.getBytes(ctx, "/api/v4/emoji/"+url.PathEscape(emojiRecord.ID)+"/image")
}

func (client Client) UserImage(ctx context.Context, userID string) ([]byte, string, error) {
	return client.getBytes(ctx, "/api/v4/users/"+url.PathEscape(userID)+"/image")
}

func (client Client) Team(ctx context.Context, name string) (string, error) {
	var team struct {
		ID string `json:"id"`
	}
	if errorValue := client.get(ctx, "/api/v4/teams/name/"+url.PathEscape(name), &team); errorValue != nil {
		return "", errorValue
	}
	if team.ID == "" {
		return "", errors.New("team not found")
	}
	return team.ID, nil
}

// Post fetches a single post by id without the type/empty filters Posts applies,
// so a reply's root can be resolved even when it is a typed (e.g. attendance
// plugin) post that the bulk import skips.
func (client Client) Post(ctx context.Context, postID string) (buzzimport.MattermostPost, bool, error) {
	var post restPost
	if errorValue := client.get(ctx, "/api/v4/posts/"+url.PathEscape(postID), &post); errorValue != nil {
		return buzzimport.MattermostPost{}, false, errorValue
	}
	if post.ID == "" || post.DeleteAt != 0 {
		return buzzimport.MattermostPost{}, false, nil
	}
	return buzzimport.MattermostPost{
		ID:           post.ID,
		ChannelID:    post.ChannelID,
		UserID:       post.UserID,
		RootID:       post.RootID,
		Message:      post.Message,
		CreatedAt:    time.UnixMilli(post.CreateAt).UTC(),
		FileIDs:      post.FileIDs,
		HasReactions: post.HasReactions,
	}, true, nil
}

// BotUserIDs returns the set of Mattermost user ids that are bots, so an import
// can attribute their posts to the shared bot buzz identity instead of skipping
// them (a bot-authored thread root left unimported strands every human reply).
func BotUserIDs(users []restUser) map[string]bool {
	botUserIDs := map[string]bool{}
	for _, user := range users {
		if user.IsBot {
			botUserIDs[user.ID] = true
		}
	}
	return botUserIDs
}

func UsersToChannelAuthorEmails(users []restUser) (map[string]string, map[string]MattermostAuthor) {
	authorEmails := map[string]string{}
	authorsByID := map[string]MattermostAuthor{}
	for _, user := range users {
		if user.IsBot {
			continue
		}
		authorEmails[user.ID] = user.Email
		authorsByID[user.ID] = MattermostAuthor{Username: user.Username, Email: user.Email, DisplayName: displayNameOf(user)}
	}
	return authorEmails, authorsByID
}

type MattermostAuthor struct {
	Username    string
	Email       string
	DisplayName string
}

func displayNameOf(user restUser) string {
	if user.Nickname != "" {
		return user.Nickname
	}
	return user.Username
}

func (channel restChannel) ToImport() buzzimport.MattermostChannel {
	return buzzimport.MattermostChannel{
		ID:          channel.ID,
		Name:        channel.Name,
		DisplayName: channel.DisplayName,
		Purpose:     channel.Purpose,
		Type:        channel.Type,
	}
}

func ChannelsToImport(channels []restChannel) []buzzimport.MattermostChannel {
	converted := make([]buzzimport.MattermostChannel, 0, len(channels))
	for _, channel := range channels {
		converted = append(converted, channel.ToImport())
	}
	return converted
}

func sortPostsByTime(posts []buzzimport.MattermostPost) {
	for outer := 1; outer < len(posts); outer++ {
		for inner := outer; inner > 0 && posts[inner-1].CreatedAt.After(posts[inner].CreatedAt); inner-- {
			posts[inner-1], posts[inner] = posts[inner], posts[inner-1]
		}
	}
}
