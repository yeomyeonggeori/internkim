# The local stack holds an ES256 signing key in the auth container's
# GOTRUE_JWT_KEYS, publishes its public half at
# /auth/v1/.well-known/jwks.json, and starts PostgREST, Realtime and Storage
# holding the same key set. `supabase status -o env` names only JWT_SECRET, so
# the private half is read off the running container. The gateway verifies only
# ES256 or RS256 against that published key set, so a host token signed with the
# HS256 JWT_SECRET is refused; handing this key to the plane as
# SUPABASE_JWT_SIGNING_KEY makes one token satisfy the gateway and row level
# security at once.
local_plane_signing_key() {
  repositoryRoot="$1"
  projectReference="$(sed -n 's/^project_id *= *"\(.*\)"$/\1/p' "$repositoryRoot/supabase/config.toml" | head -1)"
  if [ -z "$projectReference" ]; then
    echo "no project_id in $repositoryRoot/supabase/config.toml" >&2
    return 1
  fi
  keySet="$(docker inspect "supabase_auth_$projectReference" \
    --format '{{range .Config.Env}}{{println .}}{{end}}' 2>/dev/null \
    | sed -n 's/^GOTRUE_JWT_KEYS=//p' | head -1)"
  if [ -z "$keySet" ]; then
    echo "supabase_auth_$projectReference is not running, so it published no signing key" >&2
    return 1
  fi
  printf '%s' "$keySet" | python3 -c 'import json, sys; print(json.dumps(json.load(sys.stdin)[0]))'
}
