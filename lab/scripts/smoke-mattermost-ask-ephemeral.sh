#!/usr/bin/env bash
set -euo pipefail

python3 - <<'PY'
import base64
import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

tenant = os.environ.get("INTERNKIM_TENANT", "pilot-01")
mattermost_url = os.environ.get("MATTERMOST_URL", "http://127.0.0.1:18065").rstrip("/")
admind_url = os.environ.get("ADMIND_URL", "http://127.0.0.1:18180").rstrip("/")
tenant_root_path = os.environ.get("INTERNKIM_TENANT_ROOT", f"/srv/internkim/tenants/{tenant}/internkim")
capability_socket_path = os.environ.get("INTERNKIM_CAPABILITY_SOCKET", f"{tenant_root_path}/run/capability.sock")
bot_token_path = os.environ.get("MATTERMOST_BOT_TOKEN_PATH", f"{tenant_root_path}/secrets/mattermost-bot-token")
channel_id_path = os.environ.get("MATTERMOST_FLOW_CHANNEL_ID_PATH", f"{tenant_root_path}/state/admin/mattermost-flow-channel-id")

public_post_id = ""


def read_trimmed(path):
    with open(path, "r", encoding="utf-8") as file_handle:
        return file_handle.read().strip()


bot_token = read_trimmed(bot_token_path)


def mattermost_request(method, path, body=None):
    request_body = None
    headers = {"Authorization": "Bearer " + bot_token}
    if body is not None:
        request_body = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"
    request = urllib.request.Request(mattermost_url + path, data=request_body, headers=headers, method=method)
    try:
        with urllib.request.urlopen(request, timeout=10) as response:
            response_body = response.read().decode("utf-8")
            if response_body == "":
                return response.status, None
            return response.status, json.loads(response_body)
    except urllib.error.HTTPError as error:
        response_body = error.read().decode("utf-8")
        raise RuntimeError(f"{method} {path} failed {error.code}: {response_body}") from error


def resolve_channel_id():
    configured_channel_id = os.environ.get("MATTERMOST_FLOW_CHANNEL_ID", "").strip()
    if configured_channel_id != "":
        return configured_channel_id
    if os.path.isfile(channel_id_path):
        return read_trimmed(channel_id_path)
    _, team_records = mattermost_request("GET", "/api/v4/users/me/teams")
    for team_record in team_records:
        try:
            _, channel_record = mattermost_request("GET", "/api/v4/teams/" + team_record["id"] + "/channels/name/flow")
            return channel_record["id"]
        except RuntimeError:
            continue
    raise RuntimeError("Mattermost flow channel could not be resolved")


def post_json(url, body):
    request_body = json.dumps(body).encode("utf-8")
    request = urllib.request.Request(url, data=request_body, headers={"Content-Type": "application/json"}, method="POST")
    try:
        with urllib.request.urlopen(request, timeout=10) as response:
            response_body = response.read().decode("utf-8")
            if response_body == "":
                return None
            return json.loads(response_body)
    except urllib.error.HTTPError as error:
        response_body = error.read().decode("utf-8")
        raise RuntimeError(f"POST {url} failed {error.code}: {response_body}") from error


def capability_reply_send(body):
    command = [
        "curl",
        "--silent",
        "--show-error",
        "--unix-socket",
        capability_socket_path,
        "--write-out",
        "\n%{http_code}",
        "-H",
        "Content-Type: application/json",
        "-d",
        "@-",
        "http://unix/v1/platform/mattermost/reply.send",
    ]
    result = subprocess.run(command, input=json.dumps(body), text=True, capture_output=True, check=False)
    if result.returncode != 0:
        raise RuntimeError(result.stderr.strip() or result.stdout.strip() or "reply.send failed")
    response_document, separator, status_code_text = result.stdout.rpartition("\n")
    if separator == "" or not status_code_text.isdigit():
        raise RuntimeError("reply.send returned an invalid HTTP response: " + result.stdout.strip())
    status_code = int(status_code_text)
    if status_code < 200 or status_code >= 300:
        raise RuntimeError(f"reply.send failed {status_code}: {response_document.strip()}")
    return json.loads(response_document)


def encoded_reply_target_id():
    handle = {
        "platform": "mattermost",
        "conversationID": "channel:" + channel_id,
        "channelID": channel_id,
        "channelType": "O",
    }
    document = json.dumps(handle, separators=(",", ":")).encode("utf-8")
    return base64.urlsafe_b64encode(document).decode("ascii").rstrip("=")


def local_action_url(integration_url):
    parsed_url = urllib.parse.urlparse(integration_url)
    if parsed_url.path != "/_internkim/mattermost/actions":
        raise RuntimeError("ask button has an invalid integration URL: " + integration_url)
    return admind_url + parsed_url.path


def assert_wrong_user_is_rejected(action):
    integration = action.get("integration") or {}
    context = integration.get("context") or {}
    response = post_json(local_action_url(integration.get("url", "")), {
        "user_id": "wrong-user-" + bot_user["id"],
        "post_id": public_post_id,
        "channel_id": channel_id,
        "context": context,
    })
    if not response or not response.get("ephemeral_text") or response.get("update"):
        raise RuntimeError("ask button did not reject a non-target user")


def click_mattermost_action(action):
    action_id = str(action.get("id", "")).strip()
    if action_id == "":
        raise RuntimeError("ask button has no action id")
    mattermost_request("POST", "/api/v4/posts/" + public_post_id + "/actions/" + action_id, {})
    return action_id


def wait_for_attachments_to_clear():
    for _ in range(20):
        _, post = mattermost_request("GET", "/api/v4/posts/" + public_post_id)
        if not (post.get("props") or {}).get("attachments"):
            return True
        time.sleep(0.25)
    return False


def cleanup_public_post():
    global public_post_id
    if public_post_id == "":
        return False
    mattermost_request("DELETE", "/api/v4/posts/" + public_post_id)
    public_post_id = ""
    return True


try:
    _, bot_user = mattermost_request("GET", "/api/v4/users/me")
    if bot_user.get("is_bot") is not True:
        raise RuntimeError("Mattermost authentication user is not a bot")
    channel_id = resolve_channel_id()
    reply_target_id = encoded_reply_target_id()
    timestamp = str(int(time.time()))
    reply_body = {
        "replyTargetID": reply_target_id,
        "message": "codex mattermost inline ask smoke " + timestamp,
        "rawEventID": "codex-smoke-event-" + timestamp,
        "outboxID": "codex-smoke-outbox-" + timestamp,
        "interaction": {
            "interactionID": "codex-smoke-interaction-" + timestamp,
            "taskRunID": "codex-smoke-task-" + timestamp,
            "kind": "ask_choice_single",
            "question": "codex smoke",
            "options": [
                {"key": "confirm", "label": "확인"},
                {"key": "cancel", "label": "취소"},
            ],
            "selectionMode": "single",
            "responseLanguage": "ko",
            "targetPlatformUserID": bot_user["id"],
        },
    }
    reply_response = capability_reply_send(reply_body)
    reply_send_dispatch_id = reply_response.get("dispatchID", "")
    public_post_id = reply_send_dispatch_id
    if public_post_id == "":
        raise RuntimeError("reply.send did not return dispatchID")
    _, public_post = mattermost_request("GET", "/api/v4/posts/" + public_post_id)
    public_post_properties = public_post.get("props") or {}
    attachments = public_post_properties.get("attachments") or []
    if len(attachments) != 1:
        raise RuntimeError("public Mattermost post did not include one ask attachment")
    actions = attachments[0].get("actions") or []
    if len(actions) != 2:
        raise RuntimeError("public Mattermost ask did not include two choice buttons")
    selected_action = actions[0]
    selected_context = (selected_action.get("integration") or {}).get("context") or {}
    if selected_context.get("action") != "ask_choice" or selected_context.get("choiceKey") != "confirm":
        raise RuntimeError("public Mattermost ask button has the wrong action context")
    assert_wrong_user_is_rejected(selected_action)
    selected_action_id = click_mattermost_action(selected_action)
    attachments_cleared = wait_for_attachments_to_clear()
    if not attachments_cleared:
        raise RuntimeError("Mattermost ask buttons remained after a valid click")
    public_post_deleted = cleanup_public_post()
    print(json.dumps({
        "ok": True,
        "isBot": bot_user.get("is_bot") is True,
        "botRoles": bot_user.get("roles", ""),
        "publicPostDeleted": public_post_deleted,
        "publicPostInitialAttachmentCount": len(attachments),
        "publicPostAttachmentsCleared": attachments_cleared,
        "replySendDispatchID": reply_send_dispatch_id,
        "selectedActionID": selected_action_id,
        "targetMismatchRejected": True,
    }, ensure_ascii=False, sort_keys=True))
finally:
    if public_post_id != "":
        try:
            cleanup_public_post()
        except Exception as error_value:
            print("warning: failed to delete smoke post: " + str(error_value), file=sys.stderr)
PY
