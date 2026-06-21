#!/bin/bash
set -e

echo "🔦 Installing Lighthouse..."

# Dirs
mkdir -p /etc/lighthouse/ca
mkdir -p /etc/lighthouse/certs
mkdir -p /etc/lighthouse/envs
mkdir -p /var/lib/lighthouse

# Config
if [ ! -f /etc/lighthouse/config.yaml ]; then
  cp config.yml.example /etc/lighthouse/config.yaml
  echo "→ Config written to /etc/lighthouse/config.yaml"
  echo "  Edit it to set your CA paths before starting."
fi

# Build
echo "→ Building Lighthouse..."
export PATH=$PATH:/usr/local/go/bin
go build -o /usr/local/bin/lighthouse .
echo "→ Binary installed at /usr/local/bin/lighthouse"

# Systemd unit for Lighthouse itself
cat > /etc/systemd/system/lighthouse.service << 'EOF'
[Unit]
Description=Lighthouse - Pi Deploy Manager
After=network.target nginx.service

[Service]
Type=simple
ExecStart=/usr/local/bin/lighthouse
Environment=LIGHTHOUSE_CONFIG=/etc/lighthouse/config.yaml
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
