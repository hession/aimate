#!/bin/bash

# AIMate Build Script
# Usage: ./build.sh [options]
#   -o, --output    Output binary name (default: aimate)
#   -d, --debug     Build with debug symbols
#   -r, --release   Build for release (optimized)
#   -c, --clean     Clean before build
#   -a, --all       Build for all platforms
#   -h, --help      Show this help

set -e

# Default values
OUTPUT="aimate"
BUILD_DIR="bin"
DEBUG=false
RELEASE=false
CLEAN=false
ALL_PLATFORMS=false

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -o|--output)
            OUTPUT="$2"
            shift 2
            ;;
        -d|--debug)
            DEBUG=true
            shift
            ;;
        -r|--release)
            RELEASE=true
            shift
            ;;
        -c|--clean)
            CLEAN=true
            shift
            ;;
        -a|--all)
            ALL_PLATFORMS=true
            shift
            ;;
        -h|--help)
            echo "AIMate Build Script"
            echo ""
            echo "Usage: ./build.sh [options]"
            echo ""
            echo "Options:"
            echo "  -o, --output    Output binary name (default: aimate)"
            echo "  -d, --debug     Build with debug symbols"
            echo "  -r, --release   Build for release (optimized)"
            echo "  -c, --clean     Clean before build"
            echo "  -a, --all       Build for all platforms (linux, darwin, windows)"
            echo "  -h, --help      Show this help"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

# Get version info
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# LDFLAGS for version info
LDFLAGS="-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}"

# Build flags
BUILD_FLAGS=""
if [ "$DEBUG" = true ]; then
    echo -e "${YELLOW}Building with debug symbols...${NC}"
    BUILD_FLAGS="-gcflags=all=-N -l"
elif [ "$RELEASE" = true ]; then
    echo -e "${YELLOW}Building for release...${NC}"
    LDFLAGS="${LDFLAGS} -s -w"
    BUILD_FLAGS="-trimpath"
fi

# Clean if requested
if [ "$CLEAN" = true ]; then
    echo -e "${YELLOW}Cleaning build directory...${NC}"
    rm -rf "${BUILD_DIR}"
    go clean -cache
fi

# Create build directory
mkdir -p "${BUILD_DIR}"

# Build function
build() {
    local os=$1
    local arch=$2
    local output="${BUILD_DIR}/${OUTPUT}"
    
    if [ "$os" = "windows" ]; then
        output="${output}.exe"
    fi
    
    if [ "$ALL_PLATFORMS" = true ]; then
        output="${BUILD_DIR}/${OUTPUT}-${os}-${arch}"
        if [ "$os" = "windows" ]; then
            output="${output}.exe"
        fi
    fi
    
    echo -e "${YELLOW}Building for ${os}/${arch}...${NC}"
    
    GOOS=$os GOARCH=$arch go build \
        ${BUILD_FLAGS} \
        -ldflags "${LDFLAGS}" \
        -o "${output}" \
        ./cmd/aimate
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Built: ${output}${NC}"
    else
        echo -e "${RED}✗ Failed to build for ${os}/${arch}${NC}"
        return 1
    fi
}

# Run tests before build
echo -e "${YELLOW}Running tests...${NC}"
go test ./... -v
if [ $? -ne 0 ]; then
    echo -e "${RED}✗ Tests failed, aborting build${NC}"
    exit 1
fi
echo -e "${GREEN}✓ All tests passed${NC}"
echo ""

# Build
if [ "$ALL_PLATFORMS" = true ]; then
    # Build for multiple platforms
    build "darwin" "amd64"
    build "darwin" "arm64"
    build "linux" "amd64"
    build "linux" "arm64"
    build "windows" "amd64"
    
    echo ""
    echo -e "${GREEN}All builds completed!${NC}"
    ls -la "${BUILD_DIR}/"
else
    # Build for current platform
    build "$(go env GOOS)" "$(go env GOARCH)"
fi

echo ""
echo -e "${GREEN}Build completed successfully!${NC}"
