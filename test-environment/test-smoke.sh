#!/bin/bash
# Smoke Test Suite for Nigel
#
# PURPOSE:
#   Runs test scenarios and displays output for verification.
#   The observer (human or LLM) verifies the behavior is correct.
#
# USAGE:
#   ./test-smoke.sh                  # Interactive mode - pauses between tests
#   ./test-smoke.sh --non-interactive # Continuous output, no pauses
#
# Both modes run identical tests. The only difference is whether
# there are pauses between tests for manual inspection.
#
# TEST SCENARIOS:
#   1. Quick operations     - No timers should appear (fast candidate source + fast Claude)
#   2. Slow candidate source - Progress timer appears after 5 seconds
#   3. Slow Claude          - Inactivity timer appears after 30 seconds
#   4. Empty messages       - No extra blank lines in output
#   5-14. Streaming edge cases - Various chunk sizes and patterns to stress-test buffering

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
    echo -e "${GREEN}Expected: $expect${NC}"
    echo ""
    eval "env $env_vars timeout $timeout_dur $NIGEL_BIN $task" || {
        local exit_code=$?
        if [[ $exit_code -eq 124 ]]; then
            echo -e "${RED}Test timed out after ${timeout_dur}s${NC}"
        else
            echo -e "${RED}Test exited with code $exit_code${NC}"
        fi
    }
    echo ""
    echo -e "${GREEN}Test complete${NC}"
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
    echo "This script runs test scenarios for manual or automated verification."
    echo "Each test shows expected behavior - observer verifies output is correct."
    if $INTERACTIVE; then
        echo ""
        echo "Press Enter to begin..."
        read
    fi

    # Clean up before starting
    cleanup

    # Test 1: Quick operations (no timers should appear)
    run_test \
        "Test 1: Quick Operations" \
        "demo-task" \
        "No timers - candidate source and Claude both respond quickly" \
        "MOCK_CLAUDE_FIX=1 MOCK_CLAUDE_DELAY=0.5" \
        30

    cleanup

    # Test 2: Slow candidate source (progress timer after 5s)
    run_test \
        "Test 2: Slow Candidate Source" \
        "slow-candidates-task" \
        "'Running candidate source...' immediately, timer after 5 seconds" \
        "MOCK_CLAUDE_FIX=1" \
        45

    cleanup

    # Test 3: Slow Claude (inactivity timer after 30s)
    run_test \
        "Test 3: Slow Claude Response" \
        "slow-claude-task" \
        "'Waiting for Claude...' timer appears after 30 seconds of inactivity" \
        "MOCK_CLAUDE_INACTIVITY_TEST=1 MOCK_CLAUDE_FIX=1" \
        120

    cleanup

    # Test 4: Empty messages (no extra blank lines)
    run_test \
        "Test 4: Empty Messages" \
        "demo-task" \
        "No extra blank lines from empty streaming messages" \
        "MOCK_CLAUDE_EMPTY_MSG=1 MOCK_CLAUDE_FIX=1" \
        30

    cleanup

    # Edge case tests: Various chunking patterns
    run_test \
        "Test 5: Tiny Chunks (1 char)" \
        "demo-task" \
        "Each character streamed individually, no corruption or overwriting" \
        "MOCK_CLAUDE_MODE=tiny MOCK_CLAUDE_FIX=0" \
        30

    cleanup

    run_test \
        "Test 6: Small Chunks (2-5 chars)" \
        "demo-task" \
        "Chunks of 2-5 characters, tests word fragmentation" \
        "MOCK_CLAUDE_MODE=small MOCK_CLAUDE_FIX=0" \
        30

    cleanup

    run_test \
        "Test 7: Mixed Chunk Sizes" \
        "demo-task" \
        "Random chunk sizes from 1-100 characters" \
        "MOCK_CLAUDE_MODE=mixed MOCK_CLAUDE_FIX=0" \
        30

    cleanup

    run_test \
        "Test 8: Large Chunks" \
        "demo-task" \
        "Large text blocks sent in single chunks" \
        "MOCK_CLAUDE_MODE=large MOCK_CLAUDE_FIX=0" \
        30

    cleanup

    run_test \
        "Test 9: Alternating Sizes" \
        "demo-task" \
        "Tiny chunk, then large chunk, repeated" \
        "MOCK_CLAUDE_MODE=alternating MOCK_CLAUDE_FIX=0" \
        30

    cleanup

    run_test \
        "Test 10: ANSI in Chunks" \
        "demo-task" \
        "ANSI codes split across chunk boundaries" \
        "MOCK_CLAUDE_MODE=ansi MOCK_CLAUDE_FIX=0" \
        30

    cleanup

    run_test \
        "Test 11: Newline Boundaries" \
        "demo-task" \
        "Newlines at start, middle, end of chunks" \
        "MOCK_CLAUDE_MODE=newlines MOCK_CLAUDE_FIX=0" \
        30

    cleanup

    run_test \
        "Test 12: Rapid Fire" \
        "demo-task" \
        "Many tiny chunks sent with minimal delay (stress test)" \
        "MOCK_CLAUDE_MODE=rapid MOCK_CLAUDE_FIX=0" \
        30

    cleanup

    run_test \
        "Test 13: Multibyte Split" \
        "demo-task" \
        "UTF-8 sequences split across chunks" \
        "MOCK_CLAUDE_MODE=multibyte MOCK_CLAUDE_FIX=0" \
        30

    cleanup

    header "All Tests Complete"
    echo -e "${GREEN}Smoke test suite finished.${NC}"
    echo ""
    echo "Verify each test showed expected behavior:"
    echo "  1. Quick operations  - No timers appeared"
    echo "  2. Slow candidate    - Progress timer after 5s"
    echo "  3. Slow Claude       - Inactivity timer after 30s"
    echo "  4. Empty messages    - No extra blank lines"
    echo "  5-13. Edge cases     - No corruption, overwriting, or excessive spacing"
}

main "$@"
