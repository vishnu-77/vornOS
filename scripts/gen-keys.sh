#!/usr/bin/env bash
# Generate Ed25519 key pair for Vorn model signing.
# Supports key naming for rotation (e.g., vorn-sign-2025).
# Usage: ./scripts/gen-keys.sh [output-dir] [key-name]
set -euo pipefail

OUTDIR="${1:-./keys}"
KEYNAME="${2:-vorn-sign-$(date +%Y)}"
mkdir -p "$OUTDIR"

PRIVKEY="$OUTDIR/${KEYNAME}.key"
PUBKEY="$OUTDIR/${KEYNAME}.pub"

if [ -f "$PRIVKEY" ]; then
    echo "ERROR: $PRIVKEY already exists. Refusing to overwrite."
    echo "Hint: pass a unique key name, e.g.: $0 $OUTDIR vorn-sign-$(date +%Y)-$(date +%m)"
    exit 1
fi

# Generate Ed25519 key pair using OpenSSL
openssl genpkey -algorithm Ed25519 -out "$PRIVKEY"
openssl pkey -in "$PRIVKEY" -pubout -out "$PUBKEY"

chmod 600 "$PRIVKEY"
chmod 644 "$PUBKEY"

echo "Keys generated:"
echo "  Private: $PRIVKEY (keep secret — store offline!)"
echo "  Public:  $PUBKEY"
echo ""
echo "Deploy the public key:"
echo "  sudo cp $PUBKEY /etc/vorn/trust.d/"
echo "  sudo systemctl restart vornd"
