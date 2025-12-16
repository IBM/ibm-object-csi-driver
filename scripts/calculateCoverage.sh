#!/bin/bash
#******************************************************************************
# * Licensed Materials - Property of IBM
# * IBM Cloud Kubernetes Service, 5737-D43
# * (C) Copyright IBM Corp. 2025 All Rights Reserved.
# * US Government Users Restricted Rights - Use, duplication or
# * disclosure restricted by GSA ADP Schedule Contract with IBM Corp.
#******************************************************************************
#
# This script calculates the test coverage from cover.html and outputs the percentage.
#
# It is called by the GitHub Action in the pipeline to calculate the coverage percentage.

# Extract the coverage percentage from cover.html

COVERAGE=$(cat cover.html | grep "%)" | sed 's/[][()><%]/ /g' | awk '{ print $4 }' | awk '{s+=$1}END{print s/NR}')
echo "-------------------------------------------------------------------------"
echo "COVERAGE IS ${COVERAGE}%"
echo "-------------------------------------------------------------------------"