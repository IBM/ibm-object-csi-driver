#!/bin/bash
set -e
# Once package is installed this script will called to enable the service

# Reload systemd unit files
systemctl daemon-reload || true

echo "[postinst] Enabling service..."
systemctl enable cos-csi-mounter.service

echo "[postinst] Starting service..."
systemctl start cos-csi-mounter.service

echo "[postinst] Done."
