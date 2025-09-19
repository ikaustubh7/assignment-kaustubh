#!/bin/bash

# WebSocket Broadcast Server Test Runner
# This script runs all test suites for the WebSocket broadcast server

echo "🧪 Running WebSocket Broadcast Server Test Suite"
echo "================================================"

# Change to the project directory
cd "$(dirname "$0")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    if [ $1 -eq 0 ]; then
        echo -e "${GREEN}✅ $2${NC}"
    else
        echo -e "${RED}❌ $2${NC}"
        return 1
    fi
}

# Function to run a specific test suite
run_test_suite() {
    local test_name="$1"
    local test_file="$2"
    
    echo -e "\n${YELLOW}🔄 Running $test_name...${NC}"
    go test -v ./interfaces -run "$test_file" -timeout 30s
    local exit_code=$?
    print_status $exit_code "$test_name"
    return $exit_code
}

# Initialize counters
total_tests=0
passed_tests=0

# Run unit tests for Hub functionality
run_test_suite "Hub Unit Tests" "TestHub"
if [ $? -eq 0 ]; then ((passed_tests++)); fi
((total_tests++))

# Run unit tests for Client functionality  
run_test_suite "Client Unit Tests" "TestClient"
if [ $? -eq 0 ]; then ((passed_tests++)); fi
((total_tests++))

# Run HTTP API tests
run_test_suite "HTTP API Tests" "TestHub.*Handle"
if [ $? -eq 0 ]; then ((passed_tests++)); fi
((total_tests++))

# Run WebSocket integration tests
run_test_suite "WebSocket Integration Tests" "TestWebSocketIntegration"
if [ $? -eq 0 ]; then ((passed_tests++)); fi
((total_tests++))

# Run all tests together for final verification
echo -e "\n${YELLOW}🔄 Running All Tests Together...${NC}"
go test -v ./interfaces -timeout 60s
final_result=$?
print_status $final_result "All Tests Combined"

# Print summary
echo -e "\n📊 Test Summary"
echo "==============="
echo -e "Test Suites: ${passed_tests}/${total_tests} passed"

if [ $final_result -eq 0 ] && [ $passed_tests -eq $total_tests ]; then
    echo -e "${GREEN}🎉 All tests passed successfully!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed. Please check the output above.${NC}"
    exit 1
fi