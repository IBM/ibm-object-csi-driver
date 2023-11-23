#!/bin/bash
subscription-manager register --username="${RHSM_USER}" --password="${RHSM_PASS}"
subscription-manager attach --auto
