#!/bin/bash

# AIMate CLI Runner Script
# Usage: ./cli.sh [options] [aimate args]
#   -b, --build     Build before run
#   -d, --debug     Run with debug logging
#   -c, --config    Specify config directory
#   -r, --run       Run directly with go run (default)
#   -e, --exec      Run the built binary
#   -h, --help      Show this help

set -e

# Default values
BUILD_FIRST=false
DEBUG_MODE=false
CONFIG_DIR=""
RUN_MODE="run"  # run or exec
AIMATE_ARGS=""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_PATH="${SCRIPT_DIR}/bin/aimate"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -b|--build)
            BUILD_FIRST=true
            shift
            ;;
        -d|--debug)
            DEBUG_MODE=true
            shift
            ;;
        -c|--config)
            CONFIG_DIR="$2"
            shift 2
            ;;
        -r|--run)
            RUN_MODE="run"
            shift
            ;;
        -e|--exec)
            RUN_MODE="exec"
            shift
            ;;
        -h|--help)
            echo -e "${CYAN}AIMate CLI Runner Script${NC}"
            echo ""
            echo "Usage: ./cli.sh [options] [aimate args]"
            echo ""
            echo "Options:"
            echo "  -b, --build     Build before run"
            echo "  -d, --debug     Run with debug logging"
            echo "  -c, --config    Specify config directory"
            echo "  -r, --run       Run directly with go run (default)"
            echo "  -e, --exec      Run the built binary from bin/"
            echo "  -h, --help      Show this help"
            echo ""
            echo "Examples:"
            echo "  ./cli.sh                    # Run with go run"
            echo "  ./cli.sh -b -e              # Build and run binary"
            echo "  ./cli.sh -d                 # Run with debug mode"
            echo "  ./cli.sh -c ./myconfig      # Use custom config directory"
            echo "  ./cli.sh -- --help          # Pass args to aimate (optional --)"
            echo "  ./cli.sh -p \"你好\"          # Prompt mode with a quoted string"
            exit 0
            ;;
        --)
            shift
            AIMATE_ARGS="$@"
            break
            ;;
        *)
            AIMATE_ARGS="$@"
            break
            ;;
    esac
done

# Build environment variables
export AIMATE_LOG_LEVEL="info"
if [ "$DEBUG_MODE" = true ]; then
    export AIMATE_LOG_LEVEL="debug"
    echo -e "${YELLOW}Debug mode enabled${NC}"
fi

if [ -n "$CONFIG_DIR" ]; then
    export AIMATE_CONFIG_DIR="$CONFIG_DIR"
    echo -e "${YELLOW}Using config directory: ${CONFIG_DIR}${NC}"
fi

# Build if requested
if [ "$BUILD_FIRST" = true ]; then
    echo -e "${YELLOW}Building...${NC}"
    "${SCRIPT_DIR}/build.sh"
    echo ""
fi

# Run
echo -e "${CYAN}Starting AIMate...${NC}"
echo ""

if [ "$RUN_MODE" = "exec" ]; then
    # Run built binary
    if [ ! -f "$BIN_PATH" ]; then
        echo -e "${RED}Binary not found at ${BIN_PATH}${NC}"
        echo -e "${YELLOW}Run with -b flag to build first, or use -r for go run${NC}"
        exit 1
    fi
    
    exec "$BIN_PATH" $AIMATE_ARGS
else
    # Run with go run
    exec go run "${SCRIPT_DIR}/cmd/aimate/main.go" $AIMATE_ARGS
fi
