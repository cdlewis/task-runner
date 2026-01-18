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

# Run automated verification test (captures output and checks for patterns)
run_automated_test() {
    local name="$1"
    local task="$2"
    local env_vars="$3"
    local timeout_dur="${4:-60}"
    local should_have_waiting="$5"  # "true" if "Waiting for Claude" should appear, "false" if not

    header "$name"

    local log_file="test-output-$$.log"
    local test_passed=true

    echo -e "${GREEN}Running test and capturing output...${NC}"
    eval "env $env_vars timeout $timeout_dur $NIGEL_BIN $task" > "$log_file" 2>&1 || true

    echo ""
    # Check for "Waiting for Claude" pattern
    if grep -q "Waiting for Claude" "$log_file"; then
        if [ "$should_have_waiting" = "true" ]; then
            echo -e "${GREEN}✅ PASS: 'Waiting for Claude...' appeared as expected${NC}"
        else
            echo -e "${RED}❌ FAIL: 'Waiting for Claude...' appeared but should NOT have (Claude was fast!)${NC}"
            echo -e "${YELLOW}Showing matching lines:${NC}"
            grep "Waiting for Claude" "$log_file" | head -5
            test_passed=false
        fi
    else
        if [ "$should_have_waiting" = "true" ]; then
            echo -e "${RED}❌ FAIL: 'Waiting for Claude...' did NOT appear but should have (Claude was slow!)${NC}"
            echo -e "${YELLOW}Showing last 10 lines of output:${NC}"
            tail -10 "$log_file"
            test_passed=false
        else
            echo -e "${GREEN}✅ PASS: 'Waiting for Claude...' did NOT appear (correct - Claude was fast)${NC}"
        fi
    fi

    rm -f "$log_file"
    echo ""

    if [ "$test_passed" = "false" ]; then
        echo -e "${RED}Automated test FAILED${NC}"
        return 1
    fi

    return 0
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

    # If non-interactive mode, run automated verification tests first
    if ! $INTERACTIVE; then
        header "Automated Verification Tests"
        echo "Running automated tests to verify correct timer behavior..."

        local all_automated_passed=true

        # Automated Test 1: Quick Claude should NOT show "Waiting for Claude..."
        cleanup
        if ! run_automated_test \
            "Auto Test 1: Quick Claude (no waiting message)" \
            "demo-task" \
            "MOCK_CLAUDE_FIX=1 MOCK_CLAUDE_DELAY=0.5" \
            30 \
            "false"; then
            all_automated_passed=false
        fi

        # Automated Test 2: Slow Claude SHOULD show "Waiting for Claude..."
        cleanup
        if ! run_automated_test \
            "Auto Test 2: Slow Claude (waiting message expected)" \
            "slow-claude-task" \
            "MOCK_CLAUDE_INACTIVITY_TEST=1 MOCK_CLAUDE_FIX=1 MOCK_CLAUDE_DELAY=1" \
            120 \
            "true"; then
            all_automated_passed=false
        fi

        cleanup

        if [ "$all_automated_passed" = "false" ]; then
            header "Automated Tests FAILED"
            echo -e "${RED}One or more automated tests failed!${NC}"
            exit 1
        fi

        header "Automated Tests PASSED"
        echo -e "${GREEN}All automated tests passed!${NC}"
    fi

    # Test 1: Normal Behavior
    run_test \
        "Test 1: Normal Behavior" \
        "demo-task" \
        "Quick candidate source (< 5s), quick Claude response - NO timer shown" \
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
    if ! $INTERACTIVE; then
        echo "  - Automated tests: Verified timer behavior with fast/slow Claude"
    fi
    echo "  - Test 1: Verified normal behavior (no timers for quick operations)"
    echo "  - Test 2: Verified delayed progress timer for slow candidate source"
    echo "  - Test 3: Verified inactivity timer for slow Claude responses"
}

main "$@"
