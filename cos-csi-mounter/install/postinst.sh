#!/bin/bash
set -e

# Once package is installed this script will called to enable the service
systemctl enable cos-csi-mounter.service
systemctl start cos-csi-mounter.service
