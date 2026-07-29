#!/bin/bash
# Build script for Linux

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BUILD_DIR="$PROJECT_ROOT/Build"

echo "Building cygologger for Linux..."

mkdir -p "$BUILD_DIR/Bin"
mkdir -p "$BUILD_DIR/Lib"

# Build library
echo "Building Go library..."
cd "$PROJECT_ROOT/gologger"
go build -buildmode=c-archive -o "$BUILD_DIR/Lib/libgologger.a" . 2>/dev/null || true

# Build examples
echo "Building simple_example..."
cd "$PROJECT_ROOT/examples/simple_example"
go build -o "$BUILD_DIR/Bin/simple_example" .

echo "Building simple_logger_test..."
cd "$PROJECT_ROOT/examples/simple_logger_test"
go build -o "$BUILD_DIR/Bin/simple_logger_test" .

echo "Building console_test..."
cd "$PROJECT_ROOT/examples/console_test"
go build -o "$BUILD_DIR/Bin/console_test" .

echo "Building simple_test..."
cd "$PROJECT_ROOT/examples/simple_test"
go build -o "$BUILD_DIR/Bin/simple_test" .

echo ""
echo "Build artifacts:"
ls -la "$BUILD_DIR/Bin/"
