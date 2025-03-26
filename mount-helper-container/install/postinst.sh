#!/bin/bash
set -e

# Once package is installed this script will called to enable the service
systemctl enable mount-helper-container.service
systemctl start mount-helper-container.service
