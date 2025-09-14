#!/bin/sh

set -eu

PORTAINER_URL="http://localhost:9000"
ADMIN_USER=admin
ADMIN_PASS="${ADMIN_PASS:-}"
ADMIN_PASS_FILE=$(mktemp /run/admin-XXXXXXXX)

# if first arg is `-f` or `--some-option`
# (this allows for "docker run portainer --help")
if [ "${1#-}" != "$1" ]; then
    set -- /portainer "$@"
fi

if [ "${1}" != "/portainer" ]; then
    echo "DEBUG: '$1' is not a portainer command: assuming shell execution. bye." 1>&2
    exec "$@"
fi

if [ -n "${ADMIN_PASS}" ]; then
	echo "${ADMIN_PASS}" > "${ADMIN_PASS_FILE}"
	set -- "$@" "--admin-password-file=${ADMIN_PASS_FILE}"
fi

# starting portainer in background
exec "$@" &
PORTAINER_PID=$!

until curl -fs -m2 "${PORTAINER_URL}/api/system/status" > /dev/null; do
	echo "Portainer is still booting up ..."
	sleep 2
done

# enforce RequiredPasswordLength property, if ADMIN_PASS is set
if [ -n "${ADMIN_PASS}" ]; then
	PAYLOAD=$(jq -enc --arg user "${ADMIN_USER}" --arg pass "${ADMIN_PASS}" '{Username: $user, Password: $pass}')
	RESPONSE=$(curl -sf -m3 -X POST "${PORTAINER_URL}/api/auth" \
		-H "Content-Type: application/json" \
		--data "${PAYLOAD}")

	TOKEN=$(printf '%s\n' "${RESPONSE}" | jq -re '.jwt')

	PAYLOAD=$(jq -enc --arg len "${#ADMIN_PASS}" '{"InternalAuthSettings": {"RequiredPasswordLength": ($len|tonumber)}}')
	RESPONSE=$(curl -sf -m3 -X PUT "${PORTAINER_URL}/api/settings" \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer ${TOKEN}" \
		--data "${PAYLOAD}")

	printf '%s\n' "${RESPONSE}" | jq -ce .InternalAuthSettings | sed -e 's/^/Updated settings: /'

	grep -lnr 'Welcome back! Please enter your details' /public/ \
		| xargs -r -- sed -i -e "s^Welcome back! Please enter your details^Use <b>${ADMIN_USER}</b> / <b>${ADMIN_PASS}</b> for login^"

	grep -lnr 'Log in to your account' /public/ \
		| xargs -r -- sed -i -e "s^Log in to your account^=== For .local.test usage ONLY ===^"
fi

# cleanup
rm -f "${ADMIN_PASS_FILE}"

echo "Portainer is configured :)"

# keep portainer running in foreground
wait "${PORTAINER_PID}"
