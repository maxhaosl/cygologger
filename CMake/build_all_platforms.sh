#!/bin/bash
# Cross-platform build script for cygologger

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "========================================="
echo "cygologger Cross-Platform Build Script"
echo "========================================="
echo "Project root: $PROJECT_ROOT"
echo ""

OS="$(uname -s)"
case "$OS" in
    Linux*)
        echo "Detected: Linux"
        "$SCRIPT_DIR/build_linux.sh"
        ;;
    Darwin*)
        echo "Detected: macOS"
        "$SCRIPT_DIR/build_macos.sh"
        ;;
    CYGWIN*|MINGW*|MSYS*)
        echo "Detected: Windows (MinGW/MSYS)"
        "$SCRIPT_DIR/build_windows.bat"
        ;;
    *)
        echo "Unknown OS: $OS"
        exit 1
        ;;
esac

echo ""
echo "Build complete!"
echo "Output directory: $PROJECT_ROOT/Build/Bin"
