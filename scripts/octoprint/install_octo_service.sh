#!/bin/bash

# === Configuration ===
USERNAME="y7x"
OCTOPRINT_DIR="/home/$USERNAME/OctoPrint"
SERVICE_FILE="/etc/systemd/system/octoprint.service"

# === Check if executable exists ===
if [ ! -x "$OCTOPRINT_DIR/bin/octoprint" ]; then
  echo "❌ Error: OctoPrint executable not found at $OCTOPRINT_DIR/bin/octoprint"
  exit 1
fi

# === Create systemd service ===
echo "✅ Creating systemd service file at $SERVICE_FILE"

sudo tee "$SERVICE_FILE" > /dev/null <<EOF
[Unit]
Description=OctoPrint Service
After=network.target

[Service]
Type=simple
User=$USERNAME
WorkingDirectory=$OCTOPRINT_DIR
ExecStart=$OCTOPRINT_DIR/bin/octoprint serve
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

# === Reload systemd and enable the service ===
echo "🔄 Reloading systemd daemon..."
sudo systemctl daemon-reload

echo "🚀 Enabling OctoPrint service to start at boot..."
sudo systemctl enable octoprint.service

echo "🔧 Starting OctoPrint service..."
sudo systemctl start octoprint.service

echo "📋 Service status:"
sudo systemctl status octoprint.service --no-pager
