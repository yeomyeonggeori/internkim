#!/usr/bin/env bash
# Runs ON the fleet VM, appended after the helper functions of e2e-crud-remote.sh
# (the host driver strips that script's dispatch and concatenates this file).
# Drives ask.input multi-turn conversations: post a DM, detect the bot's question,
# answer it (optionally with file attachments), and assert whether later tasks
# ask again.

ask_event_count() { # task_run_id
	curl -s "$BLUECLAW/admin/api/task/detail?taskRunID=$1" | jq '[.taskEvents[]|select(.name=="ask.requested")]|length'
}

ask_question() { # task-detail-json
	printf "%s" "$1" | jq -r '[.taskEvents[]|select(.name=="ask.requested")]|last|.body // "{}"' | jq -r '.question // .message // empty'
}

find_task_for_post() { # root_post_id
	curl -s "$BLUECLAW/admin/api/task" | jq -r --arg root "$1" '[.[]|select((.originConversationID // "")|contains($root))]|sort_by(.createdAt)|last|.taskRunID // empty'
}

ask_start() { # channel_id e2e_token prompt timeout
	local channel_id="$1" e2e_token="$2" prompt="$3" timeout="${4:-300}"
	local root_post root_post_id root_post_created_at task_run_id detail status question
	root_post="$(post_channel_message "$channel_id" "$e2e_token" "$prompt")"
	root_post_id="$(printf "%s" "$root_post" | jq -r '.id // empty')"
	root_post_created_at="$(printf "%s" "$root_post" | jq -r '.create_at // 0')"
	if [ -z "$root_post_id" ]; then
		jq -cn '{phase:"post_failed"}'
		return
	fi
	task_run_id=""
	local deadline=$((SECONDS + timeout)) deadline_rebased=0
	while [ "$SECONDS" -lt "$deadline" ]; do
		if [ -z "$task_run_id" ]; then
			task_run_id="$(find_task_for_post "$root_post_id")"
			[ -z "$task_run_id" ] && { sleep 1; continue; }
			deadline=$((SECONDS + timeout))
		fi
		detail="$(curl -s "$BLUECLAW/admin/api/task/detail?taskRunID=$task_run_id")"
		status="$(printf "%s" "$detail" | jq -r '.taskRun.status')"
		case "$status" in
			waiting_user_input)
				question="$(ask_question "$detail")"
				if [ -n "$question" ]; then
					jq -cn --arg taskRunID "$task_run_id" --arg rootPostID "$root_post_id" --argjson rootPostCreatedAt "${root_post_created_at:-0}" --arg question "$question" \
						'{phase:"asked",taskRunID:$taskRunID,rootPostID:$rootPostID,rootPostCreatedAt:$rootPostCreatedAt,question:$question}'
					return
				fi
				sleep 2 ;;
			completed|failed|blocked|cancelled|canceled|error|timed_out|timeout)
				jq -cn --arg taskRunID "$task_run_id" --arg rootPostID "$root_post_id" --argjson rootPostCreatedAt "${root_post_created_at:-0}" --arg status "$status" \
					--argjson askCount "$(ask_event_count "$task_run_id")" \
					'{phase:"terminal",taskRunID:$taskRunID,rootPostID:$rootPostID,rootPostCreatedAt:$rootPostCreatedAt,status:$status,askCount:$askCount}'
				return ;;
			*) sleep 2 ;;
		esac
	done
	jq -cn --arg taskRunID "$task_run_id" --arg rootPostID "$root_post_id" '{phase:"timeout",taskRunID:$taskRunID,rootPostID:$rootPostID}'
}

upload_channel_file() { # channel_id e2e_token vm_file_path
	curl -s -H "Authorization: Bearer $2" -F "files=@$3" -F "channel_id=$1" "$MATTERMOST/api/v4/files" | jq -r '.file_infos[0].id // empty'
}

ask_answer() { # channel_id e2e_token root_post_id message [vm_file_path...]
	local channel_id="$1" e2e_token="$2" root_post_id="$3" message="$4"
	shift 4
	local file_ids="[]" file_id vm_file_path
	for vm_file_path in "$@"; do
		file_id="$(upload_channel_file "$channel_id" "$e2e_token" "$vm_file_path")"
		[ -n "$file_id" ] && file_ids="$(printf "%s" "$file_ids" | jq -c --arg id "$file_id" '. + [$id]')"
	done
	mm POST "/api/v4/posts" "$e2e_token" "$(jq -cn --arg channel_id "$channel_id" --arg message "$message" --arg root_id "$root_post_id" --argjson file_ids "$file_ids" \
		'{channel_id:$channel_id,message:$message,root_id:$root_id} + (if ($file_ids|length) == 0 then {} else {file_ids:$file_ids} end)')" | jq -c '{id:.id, fileIDs:(.file_ids // [])}'
}

ask_finish() { # channel_id e2e_token task_run_id root_post_id root_post_created_at timeout
	local channel_id="$1" e2e_token="$2" task_run_id="$3" root_post_id="$4" root_post_created_at="$5" timeout="${6:-420}"
	local detail status bot_post downloaded_files
	local deadline=$((SECONDS + timeout))
	while [ "$SECONDS" -lt "$deadline" ]; do
		detail="$(curl -s "$BLUECLAW/admin/api/task/detail?taskRunID=$task_run_id")"
		status="$(printf "%s" "$detail" | jq -r '.taskRun.status')"
		case "$status" in
			completed|failed|blocked|cancelled|canceled|error|timed_out|timeout) break ;;
			*) sleep 3 ;;
		esac
	done
	bot_post="$(latest_bot_post "$channel_id" "$root_post_id" "$root_post_created_at" "$e2e_token")"
	downloaded_files="$(download_post_files "$e2e_token" "$bot_post")"
	jq -cn --arg status "${status:-unknown}" --argjson askCount "$(ask_event_count "$task_run_id")" \
		--arg botMessage "$(printf "%s" "$bot_post" | jq -r '.message // ""')" --argjson downloadedFiles "$downloaded_files" \
		'{status:$status,askCount:$askCount,botMessage:$botMessage,downloadedFiles:$downloadedFiles}'
}

company_state() {
	local admind="http://127.0.0.1:18080"
	local requester_header="X-InternKim-Requester-Email: local-fleet-admin@internkim.test"
	jq -cn \
		--argjson info "$(curl -s -H "$requester_header" "$admind/admin/api/company-info?language=ko" || echo '{}')" \
		--argjson metrics "$(curl -s -H "$requester_header" "$admind/admin/api/company-metrics" || echo '{}')" \
		--argjson records "$(curl -s -H "$requester_header" "$admind/admin/api/company-records" || echo '{}')" \
		--argjson documents "$(curl -s -H "$requester_header" "$admind/admin/api/company-documents" || echo '{}')" \
		'{info:$info,metrics:$metrics,records:$records,documents:$documents}'
}

case "${1:-}" in
	setup) setup ;;
	ask-start) ask_start "$2" "$3" "$4" "${5:-300}" ;;
	ask-answer) shift; ask_answer "$@" ;;
	ask-finish) ask_finish "$2" "$3" "$4" "$5" "$6" "${7:-420}" ;;
	company-state) company_state ;;
	*) echo "usage: setup | ask-start <channel> <token> <prompt> [timeout] | ask-answer <channel> <token> <root_post_id> <message> [vm_file...] | ask-finish <channel> <token> <task_run_id> <root_post_id> <root_created_at> [timeout] | company-state" >&2; exit 2 ;;
esac
