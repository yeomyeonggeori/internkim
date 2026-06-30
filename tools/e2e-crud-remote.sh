#!/usr/bin/env bash
# Runs ON the fleet VM. Drives a real Mattermost conversation against 김인턴 and
# reports the real task outcome + step count as JSON. No cassettes, no scripted
# model responses: a live agent processes a real channel message.
set -euo pipefail

MATTERMOST="http://localhost:8065"
BLUECLAW="http://127.0.0.1:8080"
TEAM_NAME="internkim"
BOT_USERNAME="internkim"
NONCE="${E2E_NONCE:-x}"
E2E_USERNAME="e2ecrud$NONCE"
E2E_EMAIL="e2ecrud$NONCE@internkim.test"
E2E_PASSWORD="E2eCrud!InternKim-Mattermost"
E2E_CHANNEL_NAME="e2e-crud-$NONCE"

admin_password() { sudo cat /root/.internkim/secrets/mm-admin-pass; }

admin_token() {
	curl -s -i -d "$(jq -cn --arg login_id admin --arg password "$(admin_password)" '{login_id:$login_id,password:$password}')" \
		"$MATTERMOST/api/v4/users/login" | awk '/^[Tt]oken:/{print $2}' | tr -d '\r'
}

mm() { # method path token [body]
	local method="$1" path="$2" token="$3" body="${4:-}"
	if [ -n "$body" ]; then
		curl -s -X "$method" -H "Authorization: Bearer $token" -H "Content-Type: application/json" -d "$body" "$MATTERMOST$path"
	else
		curl -s -X "$method" -H "Authorization: Bearer $token" "$MATTERMOST$path"
	fi
}

user_id_by_name() { mm GET "/api/v4/users/username/$1" "$2" | jq -r '.id'; }

ensure_user() { # email username token [first_name] -> user_id
	local email="$1" username="$2" token="$3" first_name="${4:-}" existing
	existing="$(mm GET "/api/v4/users/username/$username" "$token" | jq -r 'if .status_code then empty else .id end')"
	if [ -n "$existing" ]; then echo "$existing"; return; fi
	mm POST "/api/v4/users" "$token" \
		"$(jq -cn --arg email "$email" --arg username "$username" --arg password "$E2E_PASSWORD" --arg first "$first_name" '{email:$email,username:$username,password:$password,first_name:$first,nickname:$first}')" \
		| jq -r '.id'
}

ensure_channel() { # team_id token -> channel_id
	local team_id="$1" token="$2" existing
	existing="$(mm GET "/api/v4/teams/$team_id/channels/name/$E2E_CHANNEL_NAME" "$token" | jq -r 'if .status_code then empty else .id end')"
	if [ -n "$existing" ]; then echo "$existing"; return; fi
	mm POST "/api/v4/channels" "$token" \
		"$(jq -cn --arg team_id "$team_id" --arg name "$E2E_CHANNEL_NAME" '{team_id:$team_id,name:$name,display_name:"E2E CRUD",type:"P"}')" \
		| jq -r '.id'
}

join_team() { mm POST "/api/v4/teams/$1/members" "$2" "$(jq -cn --arg team_id "$1" --arg user_id "$3" '{team_id:$team_id,user_id:$user_id}')" >/dev/null; }

join_channel() { mm POST "/api/v4/channels/$1/members" "$2" "$(jq -cn --arg user_id "$3" '{user_id:$user_id}')" >/dev/null; }

invite_person() { # mm_user_id email
	curl -s -X POST -H "Content-Type: application/json" "$BLUECLAW/admin/api/people/invite" \
		-d "$(jq -cn --arg personID "$1" --arg email "$2" '{personID:$personID,email:$email}')" >/dev/null || true
}

task_count() { curl -s "$BLUECLAW/admin/api/task" | jq 'length'; }

setup() {
	local token team_id channel_id bot_id e2e_id e2e_token admin_id
	token="$(admin_token)"
	team_id="$(mm GET "/api/v4/teams/name/$TEAM_NAME" "$token" | jq -r '.id')"
	admin_id="$(mm GET "/api/v4/users/me" "$token" | jq -r '.id')"
	bot_id="$(user_id_by_name "$BOT_USERNAME" "$token")"
	channel_id="$(ensure_channel "$team_id" "$token")"
	e2e_id="$(ensure_user "$E2E_EMAIL" "$E2E_USERNAME" "$token")"
	join_team "$team_id" "$token" "$e2e_id"
	join_channel "$channel_id" "$token" "$admin_id"
	join_channel "$channel_id" "$token" "$bot_id"
	join_channel "$channel_id" "$token" "$e2e_id"
	invite_person "$e2e_id" "$E2E_EMAIL"
	# DM recipient: a person resolvable by first name (테스트) for message.send cases.
	local recipient_id
	recipient_id="$(ensure_user "woojin@internkim.test" "woojin" "$token" "테스트")"
	join_team "$team_id" "$token" "$recipient_id"
	invite_person "$recipient_id" "woojin@internkim.test"
	e2e_token="$(curl -s -i -d "$(jq -cn --arg login_id "$E2E_USERNAME" --arg password "$E2E_PASSWORD" '{login_id:$login_id,password:$password}')" \
		"$MATTERMOST/api/v4/users/login" | awk '/^[Tt]oken:/{print $2}' | tr -d '\r')"
	jq -cn --arg channel_id "$channel_id" --arg e2e_token "$e2e_token" --arg channel_name "$E2E_CHANNEL_NAME" \
		--arg e2e_username "$E2E_USERNAME" --arg e2e_password "$E2E_PASSWORD" \
		'{channelID:$channel_id,e2eToken:$e2e_token,channelName:$channel_name,e2eUsername:$e2e_username,e2ePassword:$e2e_password}'
}

run_case() { # channel_id e2e_token prompt expected_op timeout
	local channel_id="$1" e2e_token="$2" prompt="$3" expected_op="$4" timeout="${5:-240}"
	local before after task_run_id detail steps status op_ok root_post_id
	before="$(task_count)"
	root_post_id="$(mm POST "/api/v4/posts" "$e2e_token" "$(jq -cn --arg channel_id "$channel_id" --arg message "$prompt" '{channel_id:$channel_id,message:$message}')" | jq -r '.id // empty')"
	for _ in $(seq 1 "$timeout"); do
		[ "$(task_count)" -ge "$((before + 1))" ] && break
		sleep 1
	done
	local confirmed=0
	for _ in $(seq 1 "$timeout"); do
		task_run_id="$(curl -s "$BLUECLAW/admin/api/task" | jq -r --arg p "$prompt" '[.[]|select(.prompt==$p)]|sort_by(.createdAt)|last|.taskRunID // empty')"
		[ -n "$task_run_id" ] || { sleep 1; continue; }
		status="$(curl -s "$BLUECLAW/admin/api/task/detail?taskRunID=$task_run_id" | jq -r '.taskRun.status')"
		case "$status" in
			running) sleep 1 ;;
			waiting_user_input|waiting_approval|blocked)
				if [ "$confirmed" = 0 ]; then
					mm POST "/api/v4/posts" "$e2e_token" "$(jq -cn --arg channel_id "$channel_id" --arg root_id "$root_post_id" --arg message "응 확인했어, 진행해줘" '{channel_id:$channel_id,root_id:$root_id,message:$message}')" >/dev/null
					confirmed=1
				fi
				sleep 2 ;;
			*) break ;;
		esac
	done
	detail="$(curl -s "$BLUECLAW/admin/api/task/detail?taskRunID=$task_run_id")"
	status="$(printf '%s' "$detail" | jq -r '.taskRun.status')"
	reason="$(printf '%s' "$detail" | jq -r '.taskRun.failureReason // ""')"
	steps="$(printf '%s' "$detail" | jq -r '[.taskSteps[]|select(.taskStepID|test("turn-"))]|length')"
	op_ok="$(printf '%s' "$detail" | jq -r '
		[.taskSteps[].output // ""]
		| (map(select(test("operation_failed|not configured|is required|capability tool is not configured")))|length) == 0')"
	jq -cn --arg id "$task_run_id" --arg status "$status" --arg reason "$reason" --argjson steps "${steps:-0}" --argjson opOk "${op_ok:-false}" \
		'{taskRunID:$id,status:$status,reason:$reason,steps:$steps,opOk:$opOk}'
}

set_model() { # model
	# capabilityd forces the upstream model via --openrouter-model (--force-openrouter-model),
	# so the model lever is the capabilityd unit, not the guest runtime.json.
	local model="$1" unit="/etc/systemd/system/internkim-capabilityd.service"
	sudo sed -i "s#--openrouter-model [^ ]*#--openrouter-model $model#" "$unit"
	sudo systemctl daemon-reload
	sudo systemctl restart internkim-capabilityd
	for _ in $(seq 1 30); do
		[ "$(systemctl is-active internkim-capabilityd 2>/dev/null)" = active ] && { echo "model set: $model"; return; }
		sleep 2
	done
	echo "model set but capabilityd not active: $model" >&2
}

case "${1:-}" in
	set-model) set_model "$2" ;;
	setup) setup ;;
	run-case) run_case "$2" "$3" "$4" "$5" "${6:-240}" ;;
	*) echo "usage: $0 set-model <model> | setup | run-case <channel_id> <e2e_token> <prompt> <expected_op> [timeout]" >&2; exit 2 ;;
esac
