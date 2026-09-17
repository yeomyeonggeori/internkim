# The local stack signs its tokens with one shared secret, so the web app
# signs record tokens with that same secret written as an oct JWK.
signing_key_of_secret() {
  printf '{"kty":"oct","k":"%s"}' "$(printf '%s' "$1" | openssl base64 -A | tr '+/' '-_' | tr -d '=')"
}
