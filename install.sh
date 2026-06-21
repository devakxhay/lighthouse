#!/bin/bash
set -e

echo "🔦 Installing Lighthouse..."

# Create group and user if they don't exist
getent group lighthouse >/dev/null || groupadd --system lighthouse
getent passwd lighthouse >/dev/null || useradd --system \
        --gid lighthouse \
        --home-dir /var/lib/lighthouse \
        --no-create-home \
        --shell /usr/sbin/nologin \
        lighthouse
# Ensure home dir is correct on re-installs / upgrades
usermod --home /var/lib/lighthouse lighthouse 2>/dev/null || true

# Add lighthouse user to systemd-journal group to read journal logs
getent group systemd-journal >/dev/null && usermod -aG systemd-journal lighthouse || true

# Dirs
CA_DIR=/etc/lighthouse/ca
mkdir -p $CA_DIR/{certs,crl,newcerts,private}
chmod 700 $CA_DIR/private
touch $CA_DIR/index.txt
touch $CA_DIR/index.txt.attr
echo 1000 > $CA_DIR/serial
echo 1000 > $CA_DIR/crlnumber

mkdir -p /etc/lighthouse/certs
mkdir -p /etc/lighthouse/envs
mkdir -p /var/lib/lighthouse
mkdir -p /var/lib/lighthouse/go          # GOPATH for go build
mkdir -p /var/lib/lighthouse/go-cache    # GOCACHE for go build

# Write openssl.cnf
cat > $CA_DIR/openssl.cnf << 'EOF'
[ ca ]
default_ca = CA_default

[ CA_default ]
dir               = /etc/lighthouse/ca
certs             = $dir/certs
crl_dir           = $dir/crl
new_certs_dir     = $dir/newcerts
database          = $dir/index.txt
serial            = $dir/serial
crlnumber         = $dir/crlnumber
private_key       = $dir/private/ca.key
certificate       = $dir/ca.crt
crl               = $dir/crl/ca.crl
crl_extensions    = crl_ext
default_crl_days  = 30
default_md        = sha256
name_opt          = ca_default
cert_opt          = ca_default
default_days      = 365
preserve          = no
policy            = policy_loose
copy_extensions   = copy

[ policy_loose ]
countryName             = optional
stateOrProvinceName     = optional
localityName            = optional
organizationName        = optional
organizationalUnitName  = optional
commonName              = supplied
emailAddress            = optional

[ req ]
default_bits        = 2048
distinguished_name  = req_distinguished_name
string_mask         = utf8only
default_md          = sha256
x509_extensions     = v3_ca

[ req_distinguished_name ]
countryName                     = Country Name (2 letter code)
stateOrProvinceName             = State or Province Name
localityName                    = Locality Name
organizationName                = Organization Name
organizationalUnitName          = Organizational Unit Name
commonName                      = Common Name

[ v3_ca ]
subjectKeyIdentifier   = hash
authorityKeyIdentifier = keyid:always,issuer
basicConstraints       = critical, CA:true
keyUsage               = critical, digitalSignature, cRLSign, keyCertSign

[ v3_req ]
basicConstraints     = CA:FALSE
keyUsage             = critical, digitalSignature, keyEncipherment
extendedKeyUsage     = serverAuth
subjectAltName       = @alt_names

[ alt_names ]
DNS.1 = placeholder

[ crl_ext ]
authorityKeyIdentifier = keyid:always,issuer
EOF

# CA key and cert generation
CA_COUNTRY=${CA_COUNTRY:-US}
CA_ORG=${CA_ORG:-Lighthouse}
CA_CN=${CA_CN:-Lighthouse CA}
CA_DAYS=${CA_DAYS:-3650}

if [ ! -f $CA_DIR/private/ca.key ]; then
  echo "→ Generating CA key..."
  openssl genrsa -out $CA_DIR/private/ca.key 4096
  chmod 600 $CA_DIR/private/ca.key
fi

if [ ! -f $CA_DIR/ca.crt ]; then
  echo "→ Generating CA self-signed certificate..."
  openssl req -new -x509 \
      -config $CA_DIR/openssl.cnf \
      -key $CA_DIR/private/ca.key \
      -out $CA_DIR/ca.crt \
      -days $CA_DAYS \
      -extensions v3_ca \
      -subj "/C=$CA_COUNTRY/O=$CA_ORG/CN=$CA_CN"
  chmod 644 $CA_DIR/ca.crt
fi

# Set ownership and permissions
chown -R lighthouse:lighthouse /etc/lighthouse
chown -R lighthouse:lighthouse /var/lib/lighthouse
chmod 750 /etc/lighthouse/ca          # CA keys — tight
if [ -f $CA_DIR/private/ca.key ]; then
  chmod 600 $CA_DIR/private/ca.key
fi

# Nginx dirs — lighthouse group gets write
chown root:lighthouse /etc/nginx/sites-available
chown root:lighthouse /etc/nginx/sites-enabled
chmod g+rwx /etc/nginx/sites-available
chmod g+rwx /etc/nginx/sites-enabled

# Systemd dir — lighthouse group gets write
chown root:lighthouse /etc/systemd/system
chmod g+wx /etc/systemd/system        # write + execute, not read (security)

# Sudoers — only specific systemctl commands
cat > /etc/sudoers.d/lighthouse << 'EOF'
lighthouse ALL=(ALL) NOPASSWD: \
    /usr/bin/systemctl daemon-reload, \
    /usr/bin/systemctl start lighthouse-*, \
    /usr/bin/systemctl stop lighthouse-*, \
    /usr/bin/systemctl restart lighthouse-*, \
    /usr/bin/systemctl enable lighthouse-*, \
    /usr/bin/systemctl disable lighthouse-*, \
    /usr/bin/systemctl reload nginx, \
    /usr/sbin/nginx -t
EOF
chmod 440 /etc/sudoers.d/lighthouse

# Config
if [ ! -f /etc/lighthouse/config.yaml ]; then
  cp config.yml.example /etc/lighthouse/config.yaml
  echo "→ Config written to /etc/lighthouse/config.yaml"
fi
chown lighthouse:lighthouse /etc/lighthouse/config.yaml

# Build
echo "→ Building Lighthouse..."
export PATH=$PATH:/usr/local/go/bin
go build -o /usr/local/bin/lighthouse .
echo "→ Binary installed at /usr/local/bin/lighthouse"
chown root:lighthouse /usr/local/bin/lighthouse
chmod 750 /usr/local/bin/lighthouse

echo "→ Building Lighthouse Install Bins Helper..."
go build -o /usr/local/bin/lighthouse-install-bins ./cmd/install-bins
echo "→ Helper binary installed at /usr/local/bin/lighthouse-install-bins"
chown root:root /usr/local/bin/lighthouse-install-bins
chmod 755 /usr/local/bin/lighthouse-install-bins

# Systemd unit for Lighthouse itself
cat > /etc/systemd/system/lighthouse.service << 'EOF'
[Unit]
Description=Lighthouse - Pi Deploy Manager
After=network.target nginx.service

[Service]
User=lighthouse
Group=lighthouse
Type=simple
ExecStart=/usr/local/bin/lighthouse
Environment=LIGHTHOUSE_CONFIG=/etc/lighthouse/config.yaml
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/local/go/bin
Environment=HOME=/var/lib/lighthouse
Environment=GOPATH=/var/lib/lighthouse/go
Environment=GOCACHE=/var/lib/lighthouse/go-cache
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable lighthouse
if systemctl is-active --quiet lighthouse; then
  systemctl restart lighthouse
else
  systemctl start lighthouse
fi

echo ""
echo "✓ Lighthouse is running at http://$(hostname -I | awk '{print $1}'):9000"
echo ""
echo "Next steps:"
echo "  1. Copy your CA cert/key to /etc/lighthouse/ca/"
echo "  2. Visit http://<pi-ip>:9000"
echo "  3. Register your first app and hit Deploy 🚀"
