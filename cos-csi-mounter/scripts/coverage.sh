COVERAGE=$(grep "%)" <cover.html | grep -v -e "fake-" | grep -v -e "metadata.go" | sed 's/[][()><%]/ /g' | awk '{ print $4 }' | awk '{s+=$1}END{print s/NR}')

echo "-------------------------------------------------------------------------"
echo "COVERAGE IS ${COVERAGE}%"
echo "-------------------------------------------------------------------------"
