#!/bin/bash
set -e

# Or condition is added to always return success even if service is not running/available.
systemctl stop cos-csi-mounter.service || true
systemctl disable cos-csi-mounter.service || true
