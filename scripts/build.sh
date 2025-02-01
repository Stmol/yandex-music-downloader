#!/bin/bash

# Exit on error
set -e

# Executable file name
APP_NAME="yamdl"

# Get version from git tag
VERSION=$(git describe --tags --always)

# Directory for builds
OUTPUT_DIR="build"
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# Target platforms
declare -a PLATFORMS=(
    "linux amd64"
    "linux arm64" 
    "windows amd64"
    "windows arm64"
    "darwin amd64"
    "darwin arm64"
)

# Build for each platform
for platform in "${PLATFORMS[@]}"; do
    IFS=' ' read -r GOOS GOARCH <<< "$platform"
    OUTPUT_NAME="$OUTPUT_DIR/${APP_NAME}"
    
    # Add .exe extension for Windows
    if [ "$GOOS" == "windows" ]; then
        OUTPUT_NAME+=".exe"
    fi
    
    echo "Building for $GOOS/$GOARCH..."
    GOOS=$GOOS GOARCH=$GOARCH go build -o "$OUTPUT_NAME" .
    
    # Zip archive
    ZIP_NAME="$OUTPUT_DIR/${APP_NAME}_${VERSION}_${GOOS}_${GOARCH}.zip"
    ZIP_NAME="${ZIP_NAME// /_}"

    zip -j "$ZIP_NAME" "$OUTPUT_NAME"
    echo "Created archive: $ZIP_NAME"

    # Remove binary after archiving
    rm "$OUTPUT_NAME"
    echo "Removed binary: $OUTPUT_NAME"
done

echo "Done"

unset GOOS
unset GOARCH