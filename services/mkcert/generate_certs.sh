#!/usr/bin/env bash

set -euo pipefail

fatal() {
    echo >&2 "Error: $*"
    false
}

generate_checksums() {
    {
        ls -1 /etc/ssl/certs/ca-certificates.crt requested_sans
        find "${TLS_PATH_CA}" -type f -name "rootCA*.pem"
        find "${TLS_PATH_CERTS}" -type f -name "proxy*"
        find "${TLS_PATH_CERTS}" -type f -name "ca-*"
    } \
        | sort -u \
        | xargs -r -- sha256sum
}

echo "Generating TLS certificates ..."

[[ -n "${TLS_PATH_CA:-}" ]] || fatal "envvar TLS_PATH_CA is empty or missing"
[[ -n "${TLS_PATH_CERTS:-}" ]] || fatal "envvar TLS_PATH_CERTS is empty or missing"
[[ -n "${TLS_SANS_COMMON:-}" ]] || fatal "envvar TLS_SANS_COMMON is empty or missing"

: "${TLS_SANS_EXTRA:=}"  # optional envvar

# store result to requested_sans
echo "${TLS_SANS_COMMON} ${TLS_SANS_EXTRA}" \
    | xargs -rn1 \
    | awk '!seen[$0]++' \
    | sed -e "/^\s*$/d" \
    | tee requested_sans \
    | xargs \
    | sed -e 's/^/SANs: /'

mkdir -p "${TLS_PATH_CA}" "${TLS_PATH_CERTS}"

export CAROOT="${TLS_PATH_CA}" TRUST_STORES="system"

if [[ ! -f "${TLS_PATH_CA}/rootCA.pem" || ! -f "${TLS_PATH_CA}/rootCA-key.pem" ]]; then
    # both files are required
    ( cd "${TLS_PATH_CA}" && rm -f rootCA*.pem )
fi

# strip empty lines from output
mkcert -install 2>&1 | sed -e '/^\s*$/d'

SUBJECT=$(openssl x509 -in "${TLS_PATH_CA}/rootCA.pem" -noout -subject)
FIPRINT=$(openssl x509 -in "${TLS_PATH_CA}/rootCA.pem" -noout -fingerprint)

echo "Local CA certificate:"
echo "  Subject     : ${SUBJECT//subject=/}"
echo "  Fingerprint : ${FIPRINT//SHA1 Fingerprint=/}"

awk -v decoder='openssl x509 -noout -subject 2>/dev/null' '/BEGIN/{close(decoder)};{print|decoder}' \
    < /etc/ssl/certs/ca-certificates.crt \
    | grep -qFx "${SUBJECT}" \
    || fatal "local CA certificate is not bundled"

if [[ -f "${TLS_PATH_CERTS}/checksums.sha256" ]]; then
    echo "Verifying existing checksums ..."

    if sha256sum -c "${TLS_PATH_CERTS}/checksums.sha256" 2>/dev/null | sed -e 's/^/  /'; then
        echo "Nothing todo - all checksums match"
        exit 0
    fi

    echo "Refreshing certificates ..."
fi

cp "${TLS_PATH_CA}/rootCA.pem" "${TLS_PATH_CERTS}/ca-proxy.crt"
cp /etc/ssl/certs/ca-certificates.crt "${TLS_PATH_CERTS}/ca-bundle.crt"

< requested_sans xargs -r \
    -- mkcert -cert-file "${TLS_PATH_CERTS}/proxy.crt" -key-file "${TLS_PATH_CERTS}/proxy.key" 2>&1 \
    | sed -e '/^\s*$/d'

generate_checksums | tee "${TLS_PATH_CERTS}/checksums.sha256" > /dev/null

echo "Local CA certificate is at \"${TLS_PATH_CERTS}/ca-proxy.crt\" and CA bundle at \"${TLS_PATH_CERTS}/ca-bundle.crt\" ✅"

echo "DONE"
