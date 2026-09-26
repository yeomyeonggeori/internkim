local_plane_signing_keys_path() {
  printf '%s/supabase/.temp/local-signing-keys.json' "$1"
}

local_plane_signing_key() {
  local keysPath
  keysPath="$(local_plane_signing_keys_path "$1")"
  if [ ! -s "$keysPath" ]; then
    echo "$keysPath does not exist; tools/with-local-plane writes it before it starts the local plane" >&2
    return 1
  fi
  python3 -c 'import json, sys; print(json.dumps(json.load(open(sys.argv[1], encoding="utf-8"))[0]))' "$keysPath"
}
