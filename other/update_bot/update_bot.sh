#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="$ROOT_DIR/.env"
LOCK_FILE="${TMPDIR:-/tmp}/proxymaster-update.lock"

cd "$ROOT_DIR"

exec 9>"$LOCK_FILE"
if ! flock -n 9; then
	printf '%s\n' "Another update is already running; skipping."
	exit 0
fi

read_env() {
	local key="$1"

	awk -v key="$key" '
		$0 ~ "^[[:space:]]*" key "[[:space:]]*=" {
			value = $0
			sub(/^[^=]*=/, "", value)
			sub(/^[[:space:]]+/, "", value)
			if (value ~ /^"/) {
				sub(/^"/, "", value)
				sub(/".*$/, "", value)
			} else if (value ~ /^'\''/) {
				sub(/^'\''/, "", value)
				sub(/'\''.*$/, "", value)
			} else {
				sub(/[[:space:]]+#.*/, "", value)
			}
			print value
			exit
		}
	' "$ENV_FILE"
}

notify_admins() {
	local text="$1"
	local admin_id
	local -a admin_ids

	if [[ -z "$TELEGRAM_TOKEN" || -z "$TELEGRAM_ADMIN_ID" ]]; then
		printf '%s\n' "Telegram credentials are missing; notification skipped." >&2
		return 0
	fi

	IFS=',' read -r -a admin_ids <<<"$TELEGRAM_ADMIN_ID"
	for admin_id in "${admin_ids[@]}"; do
		admin_id="${admin_id//[[:space:]]/}"
		if [[ ! "$admin_id" =~ ^-?[0-9]+$ ]]; then
			printf 'Ignoring invalid Telegram admin ID: %s\n' "$admin_id" >&2
			continue
		fi

		if ! curl --silent --show-error --fail --max-time 15 \
			--request POST \
			--data-urlencode "chat_id=$admin_id" \
			--data-urlencode "text=$text" \
			"https://api.telegram.org/bot${TELEGRAM_TOKEN}/sendMessage" >/dev/null; then
			printf 'Telegram notification failed for admin %s.\n' "$admin_id" >&2
		fi
	done
}

if [[ ! -f "$ENV_FILE" ]]; then
	printf 'Missing environment file: %s\n' "$ENV_FILE" >&2
	exit 1
fi

TELEGRAM_TOKEN="$(read_env TELEGRAM_TOKEN)"
TELEGRAM_ADMIN_ID="$(read_env TELEGRAM_ADMIN_ID)"

branch="$(git symbolic-ref --quiet --short HEAD)" || {
	printf '%s\n' "Cannot update from a detached HEAD." >&2
	exit 1
}

if [[ -n "$(git status --porcelain)" ]]; then
	printf '%s\n' "Working tree has local changes; refusing to pull." >&2
	notify_admins "ProxyMaster update skipped: the server repository has local changes."
	exit 1
fi

if ! git fetch --prune origin "$branch"; then
	notify_admins "ProxyMaster update failed: could not fetch origin/$branch."
	exit 1
fi

remote_ref="origin/$branch"
if ! git show-ref --verify --quiet "refs/remotes/$remote_ref"; then
	notify_admins "ProxyMaster update failed: remote branch $remote_ref was not found."
	exit 1
fi

if ! git merge-base --is-ancestor HEAD "$remote_ref"; then
	notify_admins "ProxyMaster update skipped: local branch has diverged from $remote_ref."
	exit 1
fi

new_commit_count="$(git rev-list --count "HEAD..$remote_ref")"
commit_list="$(git log --format='- %h %s' -n 10 "HEAD..$remote_ref")"
if ((new_commit_count > 10)); then
	commit_list+=$'\n'"- ... and $((new_commit_count - 10)) earlier commit(s)"
fi

if ! git pull --ff-only origin "$branch"; then
	notify_admins "ProxyMaster update failed while pulling origin/$branch."
	exit 1
fi

if ! docker compose up -d --build --force-recreate --no-deps bot; then
	notify_admins "ProxyMaster was pulled, but rebuilding or restarting the bot failed."
	exit 1
fi

if ((new_commit_count > 0)); then
	notify_admins "ProxyMaster updated and restarted with $new_commit_count new commit(s):"$'\n'"$commit_list"
else
	printf '%s\n' "No new commits; bot container was restarted."
fi
