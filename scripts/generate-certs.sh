#!/bin/bash

# TLS Certificate Generation Script for Server Manager
# This script generates self-signed certificates for testing
# For production, use certificates from a trusted CA

set -e

CERT_DIR="certs"
SERVER_KEY="$CERT_DIR/server.key"
SERVER_CERT="$CERT_DIR/server.crt"
CA_KEY="$CERT_DIR/ca.key"
CA_CERT="$CERT_DIR/ca.crt"

# Create certs directory
mkdir -p "$CERT_DIR"

echo "Generating TLS certificates..."

# Generate CA key and certificate
if [ ! -f "$CA_KEY" ]; then
    echo "Generating CA key..."
    openssl genrsa -out "$CA_KEY" 4096
fi

if [ ! -f "$CA_CERT" ]; then
    echo "Generating CA certificate..."
    openssl req -new -x509 -days 3650 -key "$CA_KEY" -out "$CA_CERT" \
        -subj "/C=US/ST=State/L=City/O=Organization/CN=Server Manager CA"
fi

# Generate server key
if [ ! -f "$SERVER_KEY" ]; then
    echo "Generating server key..."
    openssl genrsa -out "$SERVER_KEY" 2048
fi

# Generate server CSR
echo "Generating server CSR..."
openssl req -new -key "$SERVER_KEY" -out "$CERT_DIR/server.csr" \
    -subj "/C=US/ST=State/L=City/O=Organization/CN=localhost"

# Create server certificate with SAN
cat > "$CERT_DIR/server_ext.cnf" << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = *.localhost
IP.1 = 127.0.0.1
IP.2 = ::1
EOF

# Sign server certificate with CA
echo "Generating server certificate..."
openssl x509 -req -in "$CERT_DIR/server.csr" -CA "$CA_CERT" -CAkey "$CA_KEY" \
    -CAcreateserial -out "$SERVER_CERT" -days 365 \
    -extfile "$CERT_DIR/server_ext.cnf"

# Clean up
rm -f "$CERT_DIR/server.csr" "$CERT_DIR/server_ext.cnf"

echo ""
echo "Certificates generated successfully!"
echo "  CA Certificate: $CA_CERT"
echo "  Server Key: $SERVER_KEY"
echo "  Server Certificate: $SERVER_CERT"
echo ""
echo "For production use, replace with certificates from a trusted CA."
echo "To trust the CA certificate on the client:"
echo "  - Linux: Copy $CA_CERT to /usr/local/share/ca-certificates/ and run update-ca-certificates"
echo "  - macOS: Add $CA_CERT to Keychain Access and trust it"
echo "  - Windows: Import $CA_CERT to Trusted Root Certification Authorities"
