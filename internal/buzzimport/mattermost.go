package buzzimport

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type MattermostUser struct {
	ID       string
	Username string
	Email    string
	IsBot    bool
}

type MattermostChannel struct {
	ID          string
	Name        string
	DisplayName string
	Purpose     string
	Type        string
}

type MattermostPost struct {
	ID        string
	ChannelID string
	UserID    string
	RootID    string
	Message   string
	CreatedAt time.Time
	FileIDs   []string
	HasReactions bool
}

type MattermostSource struct {
	Database *sql.DB
	TeamName string
}

func (source MattermostSource) Users(ctx context.Context) ([]MattermostUser, error) {
	rows, errorValue := source.Database.QueryContext(ctx, `
		SELECT u.id, u.username, u.email, COALESCE(b.userid IS NOT NULL, false)
		FROM users u
		LEFT JOIN bots b ON b.userid = u.id
		WHERE u.deleteat = 0`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	users := []MattermostUser{}
	for rows.Next() {
		var user MattermostUser
		if errorValue := rows.Scan(&user.ID, &user.Username, &user.Email, &user.IsBot); errorValue != nil {
			return nil, errorValue
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (source MattermostSource) Channels(ctx context.Context) ([]MattermostChannel, error) {
	rows, errorValue := source.Database.QueryContext(ctx, `
		SELECT c.id, c.name, c.displayname, c.purpose, c.type
		FROM channels c
		LEFT JOIN teams t ON t.id = c.teamid
		WHERE c.deleteat = 0
		  AND c.type IN ('O', 'P')
		  AND ($1 = '' OR t.name = $1)
		ORDER BY c.createat`, source.TeamName)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	channels := []MattermostChannel{}
	for rows.Next() {
		var channel MattermostChannel
		if errorValue := rows.Scan(&channel.ID, &channel.Name, &channel.DisplayName, &channel.Purpose, &channel.Type); errorValue != nil {
			return nil, errorValue
		}
		channels = append(channels, channel)
	}
	return channels, rows.Err()
}

func (source MattermostSource) ChannelMemberEmails(ctx context.Context, channelID string) ([]string, error) {
	rows, errorValue := source.Database.QueryContext(ctx, `
		SELECT u.email
		FROM channelmembers m
		JOIN users u ON u.id = m.userid
		WHERE m.channelid = $1 AND u.deleteat = 0`, channelID)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	emails := []string{}
	for rows.Next() {
		var email string
		if errorValue := rows.Scan(&email); errorValue != nil {
			return nil, errorValue
		}
		emails = append(emails, email)
	}
	return emails, rows.Err()
}

// Posts returns the human-authored, undeleted posts of a channel oldest first,
// which is the order an import has to preserve so replies can resolve the
// event their root became.
func (source MattermostSource) Posts(ctx context.Context, channelID string) ([]MattermostPost, error) {
	rows, errorValue := source.Database.QueryContext(ctx, `
		SELECT p.id, p.channelid, p.userid, p.rootid, p.message, p.createat, COALESCE(p.fileids, '[]')
		FROM posts p
		WHERE p.channelid = $1
		  AND p.deleteat = 0
		  AND p.type = ''
		  AND p.message <> ''
		ORDER BY p.createat, p.id`, channelID)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	posts := []MattermostPost{}
	for rows.Next() {
		var post MattermostPost
		var createdAtMilliseconds int64
		var fileIDsJSON string
		if errorValue := rows.Scan(
			&post.ID, &post.ChannelID, &post.UserID, &post.RootID,
			&post.Message, &createdAtMilliseconds, &fileIDsJSON,
		); errorValue != nil {
			return nil, errorValue
		}
		post.CreatedAt = time.UnixMilli(createdAtMilliseconds).UTC()
		post.FileIDs = parseFileIDs(fileIDsJSON)
		posts = append(posts, post)
	}
	return posts, rows.Err()
}

func parseFileIDs(raw string) []string {
	if raw == "" || raw == "[]" {
		return nil
	}
	var fileIDs []string
	if errorValue := json.Unmarshal([]byte(raw), &fileIDs); errorValue != nil {
		return nil
	}
	return fileIDs
}
