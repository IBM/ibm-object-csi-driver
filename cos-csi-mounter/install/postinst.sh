#!/bin/bash
set -e

# Reload systemd unit files
systemctl daemon-reload || true

# Once package is installed this script will called to enable the service
systemctl enable cos-csi-mounter.service
systemctl start cos-csi-mounter.service
