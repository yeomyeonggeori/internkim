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
import urllib.request

tenant = os.environ.get("INTERNKIM_TENANT", "pilot-01")
mattermost_url = os.environ.get("MATTERMOST_URL", "http://127.0.0.1:18065").rstrip("/")
admind_url = os.environ.get("ADMIND_URL", "http://127.0.0.1:18180").rstrip("/")
tenant_root_path = os.environ.get("INTERNKIM_TENANT_ROOT", f"/srv/internkim/tenants/{tenant}/internkim")
capability_socket_path = os.environ.get("INTERNKIM_CAPABILITY_SOCKET", f"{tenant_root_path}/run/capability.sock")
bot_token_path = os.environ.get("MATTERMOST_BOT_TOKEN_PATH", f"{tenant_root_path}/secrets/mattermost-bot-token")
channel_id_path = os.environ.get("MATTERMOST_FLOW_CHANNEL_ID_PATH", f"{tenant_root_path}/state/admin/mattermost-flow-channel-id")
interactive_token_path = os.environ.get("MATTERMOST_INTERACTIVE_TOKEN_PATH", f"{tenant_root_path}/state/admin/mattermost-interactive-token")

public_post_id = ""


def read_trimmed(path):
    with open(path, "r", encoding="utf-8") as file_handle:
        return file_handle.read().strip()


bot_token = read_trimmed(bot_token_path)
channel_id = read_trimmed(channel_id_path)
interactive_token = read_trimmed(interactive_token_path)


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
        "--fail",
        "--unix-socket",
        capability_socket_path,
        "-H",
        "Content-Type: application/json",
        "-d",
        "@-",
        "http://unix/v1/platform/mattermost/reply.send",
    ]
    result = subprocess.run(command, input=json.dumps(body), text=True, capture_output=True, check=False)
    if result.returncode != 0:
        raise RuntimeError(result.stderr.strip() or result.stdout.strip() or "reply.send failed")
    return json.loads(result.stdout)


def encoded_reply_target_id():
    handle = {
        "platform": "mattermost",
        "conversationID": "channel:" + channel_id,
        "channelID": channel_id,
        "channelType": "O",
    }
    document = json.dumps(handle, separators=(",", ":")).encode("utf-8")
    return base64.urlsafe_b64encode(document).decode("ascii").rstrip("=")


def assert_deleted_ack(response):
    update = response.get("update")
    if not isinstance(update, dict):
        raise RuntimeError("ask ACK did not include update")
    if "ephemeral_text" in response:
        raise RuntimeError("ask ACK returned ephemeral_text")
    if update.get("message") != "":
        raise RuntimeError("ask ACK update message was not empty")
    if int(update.get("delete_at") or 0) <= 0:
        raise RuntimeError("ask ACK update did not include delete_at")
    properties = update.get("props") or {}
    if properties.get("attachments") != []:
        raise RuntimeError("ask ACK update did not clear attachments")


def cleanup_public_post():
    global public_post_id
    if public_post_id == "":
        return False
    mattermost_request("DELETE", "/api/v4/posts/" + public_post_id)
    public_post_id = ""
    return True


try:
    _, bot_user = mattermost_request("GET", "/api/v4/users/me")
    reply_target_id = encoded_reply_target_id()
    timestamp = str(int(time.time()))
    reply_body = {
        "replyTargetID": reply_target_id,
        "message": "codex mattermost ask ephemeral smoke " + timestamp,
        "rawEventID": "codex-smoke-event-" + timestamp,
        "outboxID": "codex-smoke-outbox-" + timestamp,
        "ephemeralUserID": bot_user["id"],
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
        },
    }
    reply_response = capability_reply_send(reply_body)
    reply_send_dispatch_id = reply_response.get("dispatchID", "")
    public_post_id = reply_send_dispatch_id
    if public_post_id == "":
        raise RuntimeError("reply.send did not return dispatchID")
    _, public_post = mattermost_request("GET", "/api/v4/posts/" + public_post_id)
    public_post_properties = public_post.get("props") or {}
    public_post_has_attachments = bool(public_post_properties.get("attachments"))
    if public_post_has_attachments:
        raise RuntimeError("public Mattermost post included ask attachments")
    ack_response = post_json(admind_url + "/_internkim/mattermost/actions", {
        "user_id": bot_user["id"],
        "post_id": "codex-smoke-ephemeral-post-" + timestamp,
        "channel_id": channel_id,
        "context": {
            "action": "ask.confirm",
            "token": interactive_token,
            "interactionID": "codex-smoke-interaction-" + timestamp,
            "taskRunID": "codex-smoke-task-" + timestamp,
            "conversationID": "channel:" + channel_id,
            "replyTargetID": reply_target_id,
            "responseLanguage": "ko",
        },
    })
    assert_deleted_ack(ack_response)
    public_post_deleted = cleanup_public_post()
    print(json.dumps({
        "ok": True,
        "botRoles": bot_user.get("roles", ""),
        "publicPostDeleted": public_post_deleted,
        "publicPostHasAttachments": public_post_has_attachments,
        "replySendDispatchID": reply_send_dispatch_id,
        "ackHasEphemeralText": "ephemeral_text" in ack_response,
        "ackDeleteAtPresent": int(ack_response["update"].get("delete_at") or 0) > 0,
        "ackAttachments": ack_response["update"].get("props", {}).get("attachments"),
    }, ensure_ascii=False, sort_keys=True))
finally:
    if public_post_id != "":
        try:
            cleanup_public_post()
        except Exception as error_value:
            print("warning: failed to delete smoke post: " + str(error_value), file=sys.stderr)
PY
