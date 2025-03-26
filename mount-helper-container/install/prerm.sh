#!/bin/bash
set -e

# Or condition is added to always return success even if service is not running/available.
systemctl stop mount-helper-container.service || true
systemctl disable mount-helper-container.service || true
