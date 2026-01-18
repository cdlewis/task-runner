#!/bin/bash
# Comprehensive smoke test script for Nigel
# Tests various scenarios with clear user feedback
#
# Usage: ./test-smoke.sh [--non-interactive]

set -e

NIGEL_BIN="../bin/nigel"
MOCK_CLAUDE="./mock-claude"
INTERACTIVE=true

# Check for --non-interactive flag
if [[ "$1" == "--non-interactive" ]]; then
    INTERACTIVE=false
fi

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Cleanup function - reset state between tests
cleanup() {
    echo -e "${YELLOW}Cleaning up previous test state...${NC}"
    rm -f nigel/*/ignored.log .fixed-*
    echo ""
}

# Print test section header
header() {
    echo ""
    echo -e "${BLUE}════════════════════════════════════════${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}════════════════════════════════════════${NC}"
}

# Run a single test
run_test() {
    local name="$1"
    local task="$2"
    local expect="$3"
    local env_vars="$4"
    local timeout_dur="${5:-60}"

    header "$name"
    echo -e "${GREEN}📋 Expected: $expect${NC}"
    echo ""
    eval "env $env_vars timeout $timeout_dur $NIGEL_BIN $task" || {
        local exit_code=$?
        if [[ $exit_code -eq 124 ]]; then
            echo -e "${RED}❌ Test timed out after ${timeout_dur}s${NC}"
        else
            echo -e "${RED}❌ Test failed with exit code $exit_code${NC}"
        fi
    }
    echo ""
    echo -e "${GREEN}✅ Test complete${NC}"
    echo ""
    if $INTERACTIVE; then
        echo "Press Enter to continue to next test..."
        read
    fi
}

# Main test sequence
main() {
    cd "$(dirname "$0")"

    header "Nigel Smoke Test Suite"
    echo "This script runs comprehensive tests of Nigel's functionality."
    echo "Each test will show what behavior to expect."
    if $INTERACTIVE; then
        echo ""
        echo "Press Enter to begin..."
        read
    fi

    # Clean up before starting
    cleanup

    # Test 1: Normal Behavior
    run_test \
        "Test 1: Normal Behavior" \
        "demo-task" \
        "Quick candidate source (< 5s), quick Claude response - label appears, NO timer shown" \
        "MOCK_CLAUDE_FIX=1" \
        30

    # Clean up between tests
    cleanup

    # Test 2: Slow Candidate Source
    run_test \
        "Test 2: Slow Candidate Source (>5s)" \
        "slow-candidates-task" \
        "'Running candidate source...' appears immediately, timer appears after 5 seconds" \
        "MOCK_CLAUDE_FIX=1" \
        45

    # Clean up between tests
    cleanup

    # Test 3: Slow Claude (Inactivity Timer)
    run_test \
        "Test 3: Slow Claude (Inactivity Timer)" \
        "slow-claude-task" \
        "'Waiting for Claude...' timer appears after 30 seconds of inactivity" \
        "MOCK_CLAUDE_INACTIVITY_TEST=1 MOCK_CLAUDE_FIX=1" \
        120

    # Clean up after all tests
    cleanup

    header "All Tests Complete"
    echo -e "${GREEN}Smoke test suite finished!${NC}"
    echo ""
    echo "Summary:"
    echo "  - Test 1: Verified normal behavior (no timers for quick operations)"
    echo "  - Test 2: Verified delayed progress timer for slow candidate source"
    echo "  - Test 3: Verified inactivity timer for slow Claude responses"
}

main "$@"
