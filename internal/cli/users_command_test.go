package cli

import (
	"encoding/json"
	"io"
	"testing"
)

type fakeUsersAdminAPIClient struct {
	response commandUsersResponse
	requests []fakeUsersAdminAPIRequest
}

type fakeUsersAdminAPIRequest struct {
	method string
	path   string
	body   any
}

func TestUsersAddPostsUserPayload(t *testing.T) {
	withDiscardedUsersCommandOutput(t)
	client := &fakeUsersAdminAPIClient{
		response: commandUsersResponse{
			Records: []commandUserRecord{{Email: "person@example.com", Role: "admin"}},
		},
	}

	errorValue := runUsersArgumentsWithClient([]string{
		"add",
		"--host",
		"192.0.2.10",
		"person@example.com",
		"--name",
		"Person Example",
		"--handle",
		"person",
		"--role",
		"admin",
	}, client)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(client.requests) != 1 {
		t.Fatalf("requests = %+v", client.requests)
	}
	record := client.requests[0].body.(commandUserRecord)
	if client.requests[0].method != "POST" || client.requests[0].path != "/users" {
		t.Fatalf("request = %+v", client.requests[0])
	}
	if record.Email != "person@example.com" || record.Name != "Person Example" || record.Handle != "person" || record.Role != "admin" {
		t.Fatalf("record = %+v", record)
	}
}

func TestInviteAddsMemberUser(t *testing.T) {
	withDiscardedUsersCommandOutput(t)
	client := &fakeUsersAdminAPIClient{
		response: commandUsersResponse{
			Records: []commandUserRecord{{Email: "person@example.com", Role: "member"}},
		},
	}

	errorValue := addUserWithClient([]string{"person@example.com"}, client, "member")

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	record := client.requests[0].body.(commandUserRecord)
	if record.Role != "member" {
		t.Fatalf("record = %+v", record)
	}
}

func TestUsersPromoteReusesExistingRecord(t *testing.T) {
	withDiscardedUsersCommandOutput(t)
	client := &fakeUsersAdminAPIClient{
		response: commandUsersResponse{
			Records: []commandUserRecord{{
				Email:            "person@example.com",
				Role:             "member",
				MattermostUserID: "mattermost-user",
			}},
		},
	}

	errorValue := runUsersArgumentsWithClient([]string{"promote", "person@example.com"}, client)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(client.requests) != 2 {
		t.Fatalf("requests = %+v", client.requests)
	}
	record := client.requests[1].body.(commandUserRecord)
	if record.Role != "admin" || record.MattermostUserID != "mattermost-user" {
		t.Fatalf("record = %+v", record)
	}
}

func (client *fakeUsersAdminAPIClient) request(method string, path string, requestBody any, responseBody any) ([]byte, error) {
	client.requests = append(client.requests, fakeUsersAdminAPIRequest{
		method: method,
		path:   path,
		body:   requestBody,
	})
	responseDocument, errorValue := json.Marshal(client.response)
	if errorValue != nil {
		return nil, errorValue
	}
	if responseBody != nil {
		if errorValue := json.Unmarshal(responseDocument, responseBody); errorValue != nil {
			return nil, errorValue
		}
	}
	return responseDocument, nil
}

func withDiscardedUsersCommandOutput(t *testing.T) {
	t.Helper()
	previousOutput := usersCommandOutput
	usersCommandOutput = io.Discard
	t.Cleanup(func() {
		usersCommandOutput = previousOutput
	})
}
