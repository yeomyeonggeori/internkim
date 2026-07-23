#!/usr/bin/env bash
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

# The local fleet is reached at the VM IP, but provisioning sets AllowCorsFrom to
# the cloudflare SiteURL, so Mattermost rejects the webapp websocket origin (1006)
# and never delivers live events (ephemeral approval controls included). Allow any
# origin so a local browser session receives them, matching real cloudflare access.
# Name-resolved entities (업무/일정) accumulate across runs and users, so a delete
# query can resolve a different requester's same-named entity and fail the owner
# check. Reset the entity stores once per suite so each run resolves deterministically.
reset_entity_state() {
	sudo systemctl stop internkim-admind 2>/dev/null || true
	sudo rm -f /root/.internkim/state/flow.sqlite /root/.internkim/state/calendar.sqlite
	sudo systemctl start internkim-admind 2>/dev/null || true
	local attempt
	for attempt in $(seq 1 20); do
		[ "$(systemctl is-active internkim-admind 2>/dev/null)" = active ] && return
		sleep 1
	done
}

ensure_websocket_cors() { # token
	local current
	current="$(mm GET /api/v4/config "$1" | jq -r '.ServiceSettings.AllowCorsFrom')"
	[ "$current" = "*" ] && return
	mm GET /api/v4/config "$1" | jq '.ServiceSettings.AllowCorsFrom="*"' > /tmp/e2e-mmconfig.json
	mm PUT /api/v4/config "$1" "$(cat /tmp/e2e-mmconfig.json)" >/dev/null
}

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

ensure_user() { # email username token [first_name] [last_name] [nickname] -> user_id
	local email="$1" username="$2" token="$3" first_name="${4:-}" last_name="${5:-}" nickname="${6:-${4:-}}" existing
	existing="$(mm GET "/api/v4/users/username/$username" "$token" | jq -r 'if .status_code then empty else .id end')"
	if [ -n "$existing" ]; then echo "$existing"; return; fi
	mm POST "/api/v4/users" "$token" \
		"$(jq -cn --arg email "$email" --arg username "$username" --arg password "$E2E_PASSWORD" --arg first "$first_name" --arg last "$last_name" --arg nick "$nickname" '{email:$email,username:$username,password:$password,first_name:$first,last_name:$last,nickname:$nick}')" \
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

ensure_direct_channel() { # token first_user_id second_user_id -> channel_id
	mm POST "/api/v4/channels/direct" "$1" "$(jq -cn --arg first "$2" --arg second "$3" '[$first,$second]')" | jq -r '.id'
}

invite_person() { # mm_user_id email [display_name]
	local response_file status_code
	response_file="$(mktemp)"
	if ! status_code="$(curl -s -o "$response_file" -w "%{http_code}" -X POST -H "Content-Type: application/json" "$BLUECLAW/admin/api/people/invite" \
		-d "$(jq -cn --arg personID "$1" --arg email "$2" --arg name "${3:-}" \
			'{personID:$personID,email:$email} + (if $name == "" then {} else {displayName:$name} end)')")"; then
		echo "Blueclaw invite request failed for $2" >&2
		rm -f "$response_file"
		return 1
	fi
	if [ "$status_code" -lt 200 ] || [ "$status_code" -ge 300 ]; then
		echo "Blueclaw invite failed for $2 ($status_code): $(cat "$response_file")" >&2
		rm -f "$response_file"
		return 1
	fi
	rm -f "$response_file"
}

assert_invited_person() { # email
	curl -s "$BLUECLAW/admin/api/policy" | jq -e --arg email "$1" 'any(.people[]?; any((.emails // [])[]?; . == $email))' >/dev/null
}

task_count() { curl -s "$BLUECLAW/admin/api/task" | jq 'length'; }

post_channel_message() { # channel_id token message [root_post_id]
	local channel_id="$1" token="$2" message="$3" root_post_id="${4:-}"
	mm POST "/api/v4/posts" "$token" "$(jq -cn --arg channel_id "$channel_id" --arg message "$message" --arg root_id "$root_post_id" '
		{channel_id:$channel_id,message:$message} + (if $root_id == "" then {} else {root_id:$root_id} end)')"
}

latest_bot_post() { # channel_id root_post_id root_post_created_at token
	local channel_id="$1" root_post_id="$2" root_post_created_at="$3" token="$4" bot_id thread post
	bot_id="$(user_id_by_name "$BOT_USERNAME" "$token")"
	thread="$(mm GET "/api/v4/posts/$root_post_id/thread" "$token")"
	post="$(printf "%s" "$thread" | jq -c --arg bot_id "$bot_id" '[((.posts // {})[]) | select(.user_id == $bot_id)] | sort_by(.create_at) | last // {}')"
	if [ "$(printf "%s" "$post" | jq -r '.id // empty')" != "" ]; then
		printf "%s" "$post"
		return
	fi
	mm GET "/api/v4/channels/$channel_id/posts?per_page=80" "$token" | jq -c --arg bot_id "$bot_id" --argjson posted_after "${root_post_created_at:-0}" \
		'[.posts[] | select(.user_id == $bot_id and (.create_at >= $posted_after))] | sort_by(.create_at) | last // {}'
}

download_post_files() { # token post-json
	local token="$1" post_json="$2" downloaded_files_file file_id file_info filename content_type attachment_file content_base64_file next_downloaded_files_file
	downloaded_files_file="$(mktemp)"
	printf '[]' > "$downloaded_files_file"
	for file_id in $(printf "%s" "$post_json" | jq -r '.file_ids[]?'); do
		file_info="$(mm GET "/api/v4/files/$file_id/info" "$token")"
		filename="$(printf "%s" "$file_info" | jq -r --arg file_id "$file_id" '.name // .filename // $file_id')"
		content_type="$(printf "%s" "$file_info" | jq -r '.mime_type // .content_type // ""')"
		attachment_file="$(mktemp)"
		content_base64_file="$(mktemp)"
		if ! curl -s --fail -H "Authorization: Bearer $token" "$MATTERMOST/api/v4/files/$file_id" -o "$attachment_file"; then
			rm -f "$attachment_file" "$content_base64_file"
			continue
		fi
		base64 -w 0 "$attachment_file" > "$content_base64_file" 2>/dev/null || base64 "$attachment_file" | tr -d '\n' > "$content_base64_file"
		next_downloaded_files_file="$(mktemp)"
		jq \
			--arg file_id "$file_id" \
			--arg filename "$filename" \
			--arg content_type "$content_type" \
			--rawfile content_base64 "$content_base64_file" \
			'. + [{fileID:$file_id, filename:$filename, contentType:$content_type, contentBase64:$content_base64}]' \
			"$downloaded_files_file" > "$next_downloaded_files_file"
		mv "$next_downloaded_files_file" "$downloaded_files_file"
		rm -f "$attachment_file" "$content_base64_file"
	done
	cat "$downloaded_files_file"
	rm -f "$downloaded_files_file"
}

extract_public_url() { # bot-post-json task-detail-json
	local bot_post="$1" detail="$2" public_url
	public_url="$(printf "%s" "$detail" | jq -r '[.taskEvents[]? | select(.name == "tool.capability.invoke.result") | (.body | fromjson? // {}) | .output.content // "" | fromjson? // {} | .publishedURL // empty] | last // empty' 2>/dev/null || true)"
	if [ -n "$public_url" ]; then
		printf "%s" "$public_url"
		return
	fi
	public_url="$(printf "%s" "$bot_post" | jq -r '.message // ""' | grep -Eo 'https?://[^[:space:])>]+' | sed -E 's/[).,;:!?*\\"]+$//' | grep -E 'intern\.kim|localhost|127\.0\.0\.1' | grep -Ev '/tasks/|/login|/flow/|:8065' | head -1 || true)"
	if [ -n "$public_url" ]; then
		printf "%s" "$public_url"
		return
	fi
	printf "%s" "$detail" | jq -r '.. | strings' | grep -Eo 'https?://[^[:space:])>]+' | sed -E 's/[).,;:!?*\\"]+$//' | grep -E 'intern\.kim|localhost|127\.0\.0\.1' | grep -Ev '/tasks/|/login|/flow/|:8065' | head -1 || true
}

pending_approval_operation() { # task-detail-json
	printf "%s" "$1" | jq -r '
		[.taskEvents[]? | select(.name == "approval.pending_call") | (.body | fromjson? // {})] |
		last |
		(.toolInput.operation // .toolName // "")'
}

setup() {
	local token team_id channel_id direct_channel_id bot_id e2e_id e2e_token admin_id
	# Skip the global admind/entity reset for parallel-safe runs (a fresh person+channel
	# per run already isolates state; the reset restarts admind and would stomp peers).
	[ "${E2E_SKIP_RESET:-}" = 1 ] || reset_entity_state
	token="$(admin_token)"
	ensure_websocket_cors "$token"
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
	assert_invited_person "$E2E_EMAIL"
	# DM recipient: 홍길동, resolvable by the given name 길동 for message.send cases.
	local recipient_id
	recipient_id="$(ensure_user "gildong@internkim.test" "gildong" "$token" "길동" "홍" "홍길동")"
	join_team "$team_id" "$token" "$recipient_id"
	invite_person "$recipient_id" "gildong@internkim.test" "홍길동"
	assert_invited_person "gildong@internkim.test"
	e2e_token="$(curl -s -i -d "$(jq -cn --arg login_id "$E2E_USERNAME" --arg password "$E2E_PASSWORD" '{login_id:$login_id,password:$password}')" \
		"$MATTERMOST/api/v4/users/login" | awk '/^[Tt]oken:/{print $2}' | tr -d '\r')"
	direct_channel_id="$(ensure_direct_channel "$e2e_token" "$e2e_id" "$bot_id")"
	jq -cn --arg channel_id "$direct_channel_id" --arg e2e_token "$e2e_token" --arg channel_name "$E2E_CHANNEL_NAME" --arg channel_path "/$TEAM_NAME/messages/@$BOT_USERNAME" \
		--arg e2e_username "$E2E_USERNAME" --arg e2e_password "$E2E_PASSWORD" \
		'{channelID:$channel_id,e2eToken:$e2e_token,channelName:$channel_name,channelPath:$channel_path,e2eUsername:$e2e_username,e2ePassword:$e2e_password}'
}

post_approval_reply() { # channel_id e2e_token root_post_id
	post_channel_message "$1" "$2" "@$BOT_USERNAME 응 확인했어, 진행해줘" "$3" >/dev/null
}

# Builds the same result JSON shape run_case has always returned, from a task_run_id
# that has already reached (or timed out short of) a terminal status. Shared by
# run_case (single-shot, non-approval cases) and run_case_finish (the second half of
# the resumable approval flow) so the result contract never drifts between the two.
build_case_result() { # channel_id e2e_token task_run_id root_post_id root_post_created_at expected_op
	local channel_id="$1" e2e_token="$2" task_run_id="$3" root_post_id="$4" root_post_created_at="$5" expected_op="$6"
	local bot_post file_ids downloaded_files public_url bot_post_id bot_message detail status reason steps expected_operation_observed expected_operation_succeeded recovered_error_count op_ok terminal_run_count
	bot_post="{}"
	for _ in $(seq 1 20); do
		bot_post="$(latest_bot_post "$channel_id" "$root_post_id" "$root_post_created_at" "$e2e_token")"
		[ "$(printf "%s" "$bot_post" | jq -r '.id // empty')" != "" ] && break
		sleep 1
	done
	bot_post_id="$(printf "%s" "$bot_post" | jq -r '.id // empty')"
	bot_message="$(printf "%s" "$bot_post" | jq -r '.message // ""')"
	file_ids="$(printf "%s" "$bot_post" | jq -c '.file_ids // []')"
	downloaded_files="$(download_post_files "$e2e_token" "$bot_post")"
	if [ -z "$task_run_id" ]; then
		jq -cn --arg status "not_started" --arg reason "no Blueclaw task was created from Mattermost message before timeout" --arg expectedOp "$expected_op" --arg botPostID "$bot_post_id" --arg botMessage "$bot_message" --argjson fileIDs "${file_ids:-[]}" --argjson downloadedFiles "${downloaded_files:-[]}" \
			'{taskRunID:"",status:$status,reason:$reason,steps:0,expectedOp:$expectedOp,expectedOpObserved:false,opOk:false,publicURL:"",botPostID:$botPostID,botMessage:$botMessage,fileIDs:$fileIDs,downloadedFiles:$downloadedFiles,terminalRunCount:0,recoveredErrorCount:0}'
		return
	fi
	detail="$(curl -s "$BLUECLAW/admin/api/task/detail?taskRunID=$task_run_id")"
	if ! printf "%s" "$detail" | jq -e . >/dev/null 2>&1; then
		jq -cn --arg id "$task_run_id" --arg status "error" --arg reason "Blueclaw task detail returned non-JSON" --arg expectedOp "$expected_op" --arg botPostID "$bot_post_id" --arg botMessage "$bot_message" --argjson fileIDs "${file_ids:-[]}" --argjson downloadedFiles "${downloaded_files:-[]}" \
			'{taskRunID:$id,status:$status,reason:$reason,steps:0,expectedOp:$expectedOp,expectedOpObserved:false,opOk:false,publicURL:"",botPostID:$botPostID,botMessage:$botMessage,fileIDs:$fileIDs,downloadedFiles:$downloadedFiles,terminalRunCount:0,recoveredErrorCount:0}'
		return
	fi
	status="$(printf '%s' "$detail" | jq -r '.taskRun.status')"
	reason="$(printf '%s' "$detail" | jq -r '.taskRun.failureReason // ""')"
	steps="$(printf '%s' "$detail" | jq -r '[.taskSteps[]|select(.taskStepID|test("turn-"))]|length')"
	expected_operation_observed="$(printf '%s' "$detail" | jq -r --arg expected_op "$expected_op" '
		any((.taskEvents // [])[];
			(.name == ("tool." + $expected_op + ".requested")) or
			(.name == "tool.capability.invoke.requested" and ((.body // "") | tostring | contains("\"operation\":\"" + $expected_op + "\"")))
		)')"
	# The case passes on final state: an earlier transient tool error that the
	# agent recovers from must not fail a case whose expected operation went on
	# to succeed. A success is a tool.capability.invoke.result (or a direct
	# tool.<op>.result) event whose observation names the expected operation and
	# carries no failure — see turnObservation.tool/turnObservation.failure in
	# .dependency/blueclaw/internal/agent/turn_runner.go.
	expected_operation_succeeded="$(printf '%s' "$detail" | jq -r --arg expected_op "$expected_op" '
		any((.taskEvents // [])[];
			(.name == "tool.capability.invoke.result" or .name == ("tool." + $expected_op + ".result")) and
			((.body | fromjson? // {}) as $observation | $observation.tool == $expected_op and ($observation.failure // null) == null)
		)')"
	recovered_error_count="$(printf '%s' "$detail" | jq -r '
		[.taskSteps[].output // ""]
		| map(select(test("operation_failed|not configured|is required|capability tool is not configured")))
		| length')"
	op_ok="$expected_operation_succeeded"
	public_url="$(extract_public_url "$bot_post" "$detail")"
	terminal_run_count="$(printf '%s' "$detail" | jq -r '[(.taskEvents // [])[]|select(.name == "tool.terminal.run.requested")]|length')"
	jq -cn --arg id "$task_run_id" --arg status "$status" --arg reason "$reason" --arg expectedOp "$expected_op" --arg publicURL "$public_url" --arg botPostID "$bot_post_id" --arg botMessage "$bot_message" --argjson steps "${steps:-0}" --argjson opOk "${op_ok:-false}" --argjson expectedOpObserved "${expected_operation_observed:-false}" --argjson fileIDs "${file_ids:-[]}" --argjson downloadedFiles "${downloaded_files:-[]}" --argjson terminalRunCount "${terminal_run_count:-0}" --argjson recoveredErrorCount "${recovered_error_count:-0}" \
		'{taskRunID:$id,status:$status,reason:$reason,steps:$steps,expectedOp:$expectedOp,expectedOpObserved:$expectedOpObserved,opOk:$opOk,publicURL:$publicURL,botPostID:$botPostID,botMessage:$botMessage,fileIDs:$fileIDs,downloadedFiles:$downloadedFiles,terminalRunCount:$terminalRunCount,recoveredErrorCount:$recoveredErrorCount}'
}

run_case() { # channel_id e2e_token prompt expected_op timeout
	local channel_id="$1" e2e_token="$2" prompt="$3" expected_op="$4" timeout="${5:-240}"
	local task_run_id detail status root_post_id root_post root_post_created_at pending_operation attempt message
	message="@$BOT_USERNAME $prompt"
	root_post="$(post_channel_message "$channel_id" "$e2e_token" "$message")"
	root_post_id="$(printf "%s" "$root_post" | jq -r '.id // empty')"
	root_post_created_at="$(printf "%s" "$root_post" | jq -r '.create_at // 0')"
	if [ -z "$root_post_id" ]; then
		jq -cn --arg status "error" --arg reason "failed to post Mattermost message" --arg expectedOp "$expected_op" \
			'{taskRunID:"",status:$status,reason:$reason,steps:0,expectedOp:$expectedOp,expectedOpObserved:false,opOk:false,publicURL:"",botPostID:"",botMessage:"",fileIDs:[],downloadedFiles:[],terminalRunCount:0,recoveredErrorCount:0}'
		return
	fi
	local confirmed=0
	local deadline=$((SECONDS + timeout))
	while [ "$SECONDS" -lt "$deadline" ]; do
		task_run_id="$(curl -s "$BLUECLAW/admin/api/task" | jq -r --arg root "$root_post_id" '[.[]|select((.originConversationID // "")|contains($root))]|sort_by(.createdAt)|last|.taskRunID // empty')"
		if [ -z "$task_run_id" ]; then
			sleep 1
			continue
		fi
		if [ "${deadline_rebased:-0}" = 0 ]; then
			deadline=$((SECONDS + timeout))
			deadline_rebased=1
		fi
		detail="$(curl -s "$BLUECLAW/admin/api/task/detail?taskRunID=$task_run_id")"
		status="$(printf "%s" "$detail" | jq -r '.taskRun.status')"
		case "$status" in
			completed|failed|cancelled|canceled|error|timed_out|timeout) break ;;
			blocked)
				pending_operation="$(pending_approval_operation "$detail")"
				[ -n "$pending_operation" ] || break
				if [ "$confirmed" = 0 ]; then
					[ "$pending_operation" = "$expected_op" ] || break
					post_approval_reply "$channel_id" "$e2e_token" "$root_post_id"
					confirmed=1
				fi
				sleep 2 ;;
			waiting_user_input|waiting_approval)
				if [ "$confirmed" = 0 ]; then
					pending_operation="$(pending_approval_operation "$detail")"
					[ -z "$pending_operation" ] || [ "$pending_operation" = "$expected_op" ] || break
					post_approval_reply "$channel_id" "$e2e_token" "$root_post_id"
					confirmed=1
				fi
				sleep 2 ;;
			*) sleep 1 ;;
		esac
	done
	build_case_result "$channel_id" "$e2e_token" "$task_run_id" "$root_post_id" "$root_post_created_at" "$expected_op"
}

# Phase 1 of the resumable approval flow: post the prompt and poll until either a
# specific approval is pending (reuses pending_approval_operation, same signal
# run_case uses to decide whether to auto-confirm), the task reaches a terminal
# status without ever asking (a destructive op skipping approval — a bug to catch),
# or the poll window times out before either happens.
run_case_start() { # channel_id e2e_token prompt timeout
	local channel_id="$1" e2e_token="$2" prompt="$3" timeout="${4:-240}"
	local message root_post root_post_id root_post_created_at task_run_id detail status pending_operation attempt
	message="@$BOT_USERNAME $prompt"
	root_post="$(post_channel_message "$channel_id" "$e2e_token" "$message")"
	root_post_id="$(printf "%s" "$root_post" | jq -r '.id // empty')"
	root_post_created_at="$(printf "%s" "$root_post" | jq -r '.create_at // 0')"
	if [ -z "$root_post_id" ]; then
		jq -cn '{phase:"not_started",taskRunID:"",rootPostID:"",rootPostCreatedAt:0,pendingOperation:""}'
		return
	fi
	task_run_id=""
	local deadline=$((SECONDS + timeout))
	while [ "$SECONDS" -lt "$deadline" ]; do
		task_run_id="$(curl -s "$BLUECLAW/admin/api/task" | jq -r --arg root "$root_post_id" '[.[]|select((.originConversationID // "")|contains($root))]|sort_by(.createdAt)|last|.taskRunID // empty')"
		if [ -z "$task_run_id" ]; then
			sleep 1
			continue
		fi
		if [ "${deadline_rebased:-0}" = 0 ]; then
			deadline=$((SECONDS + timeout))
			deadline_rebased=1
		fi
		detail="$(curl -s "$BLUECLAW/admin/api/task/detail?taskRunID=$task_run_id")"
		status="$(printf "%s" "$detail" | jq -r '.taskRun.status')"
		case "$status" in
			completed|failed|cancelled|canceled|error|timed_out|timeout)
				jq -cn --arg taskRunID "$task_run_id" --arg rootPostID "$root_post_id" --argjson rootPostCreatedAt "${root_post_created_at:-0}" \
					'{phase:"terminal",taskRunID:$taskRunID,rootPostID:$rootPostID,rootPostCreatedAt:$rootPostCreatedAt,pendingOperation:""}'
				return ;;
			blocked)
				pending_operation="$(pending_approval_operation "$detail")"
				if [ -n "$pending_operation" ]; then
					jq -cn --arg taskRunID "$task_run_id" --arg rootPostID "$root_post_id" --argjson rootPostCreatedAt "${root_post_created_at:-0}" --arg pendingOperation "$pending_operation" \
						'{phase:"approval_pending",taskRunID:$taskRunID,rootPostID:$rootPostID,rootPostCreatedAt:$rootPostCreatedAt,pendingOperation:$pendingOperation}'
					return
				fi
				jq -cn --arg taskRunID "$task_run_id" --arg rootPostID "$root_post_id" --argjson rootPostCreatedAt "${root_post_created_at:-0}" \
					'{phase:"terminal",taskRunID:$taskRunID,rootPostID:$rootPostID,rootPostCreatedAt:$rootPostCreatedAt,pendingOperation:""}'
				return ;;
			waiting_user_input|waiting_approval)
				pending_operation="$(pending_approval_operation "$detail")"
				if [ -n "$pending_operation" ]; then
					jq -cn --arg taskRunID "$task_run_id" --arg rootPostID "$root_post_id" --argjson rootPostCreatedAt "${root_post_created_at:-0}" --arg pendingOperation "$pending_operation" \
						'{phase:"approval_pending",taskRunID:$taskRunID,rootPostID:$rootPostID,rootPostCreatedAt:$rootPostCreatedAt,pendingOperation:$pendingOperation}'
					return
				fi
				sleep 2 ;;
			*) sleep 1 ;;
		esac
	done
	jq -cn --arg taskRunID "$task_run_id" --arg rootPostID "$root_post_id" --argjson rootPostCreatedAt "${root_post_created_at:-0}" \
		'{phase:"not_started",taskRunID:$taskRunID,rootPostID:$rootPostID,rootPostCreatedAt:$rootPostCreatedAt,pendingOperation:""}'
}

# Phase 2: post the approval reply in-thread, nothing else. The caller takes the
# BEFORE screenshot between run_case_start returning approval_pending and calling
# this, so the approval question is still visible when captured.
run_case_approve() { # channel_id e2e_token root_post_id
	post_approval_reply "$1" "$2" "$3"
}

# Phase 3: resume polling an already-approved task to a terminal status, then build
# the same result JSON run_case returns for the non-approval path.
run_case_finish() { # channel_id e2e_token task_run_id root_post_id root_post_created_at expected_op timeout
	local channel_id="$1" e2e_token="$2" task_run_id="$3" root_post_id="$4" root_post_created_at="$5" expected_op="$6" timeout="${7:-240}"
	local detail status attempt
	local deadline=$((SECONDS + timeout))
	while [ "$SECONDS" -lt "$deadline" ]; do
		detail="$(curl -s "$BLUECLAW/admin/api/task/detail?taskRunID=$task_run_id")"
		status="$(printf "%s" "$detail" | jq -r '.taskRun.status')"
		case "$status" in
			completed|failed|cancelled|canceled|error|timed_out|timeout) break ;;
			*) sleep 2 ;;
		esac
	done
	build_case_result "$channel_id" "$e2e_token" "$task_run_id" "$root_post_id" "$root_post_created_at" "$expected_op"
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

restart_blueclaw() {
	sudo systemctl restart blueclaw
	local attempt
	for attempt in $(seq 1 60); do
		if [ "$(systemctl is-active blueclaw 2>/dev/null)" = active ] && curl -fsS "$BLUECLAW/admin/api/policy" >/dev/null 2>&1; then
			echo "blueclaw restarted"
			return
		fi
		sleep 2
	done
	echo "blueclaw restart did not become active" >&2
	return 1
}

find_status() { # prompt -> latest matching task run status + steps
	local prompt="$1" task_run_id detail
	task_run_id="$(curl -s "$BLUECLAW/admin/api/task" | jq -r --arg p "$prompt" '[.[]|select(.prompt==$p)]|sort_by(.createdAt)|last|.taskRunID // empty')"
	if [ -z "$task_run_id" ]; then echo '{"status":"none","steps":0}'; return; fi
	detail="$(curl -s "$BLUECLAW/admin/api/task/detail?taskRunID=$task_run_id")"
	jq -cn --arg id "$task_run_id" \
		--arg status "$(printf '%s' "$detail" | jq -r '.taskRun.status')" \
		--arg reason "$(printf '%s' "$detail" | jq -r '.taskRun.failureReason // ""')" \
		--argjson steps "$(printf '%s' "$detail" | jq '[.taskSteps[]|select(.taskStepID|test("turn-"))]|length')" \
		'{taskRunID:$id,status:$status,reason:$reason,steps:$steps}'
}

case "${1:-}" in
	set-model) set_model "$2" ;;
	restart-blueclaw) restart_blueclaw ;;
	setup) setup ;;
	run-case) run_case "$2" "$3" "$4" "$5" "${6:-240}" ;;
	run-case-start) run_case_start "$2" "$3" "$4" "${5:-240}" ;;
	run-case-approve) run_case_approve "$2" "$3" "$4" ;;
	run-case-finish) run_case_finish "$2" "$3" "$4" "$5" "$6" "$7" "${8:-240}" ;;
	find-status) find_status "$2" ;;
	*) echo "usage: $0 set-model <model> | restart-blueclaw | setup | run-case <channel_id> <e2e_token> <prompt> <expected_op> [timeout] | run-case-start <channel_id> <e2e_token> <prompt> [timeout] | run-case-approve <channel_id> <e2e_token> <root_post_id> | run-case-finish <channel_id> <e2e_token> <task_run_id> <root_post_id> <root_post_created_at> <expected_op> [timeout] | find-status <prompt>" >&2; exit 2 ;;
esac
