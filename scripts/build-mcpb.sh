#!/bin/bash
# Build MCPB bundles for all platforms
# Usage: ./scripts/build-mcpb.sh [version]

set -e

VERSION="${1:-0.5.0}"
DIST_DIR="dist"
MCPB_DIR="dist/mcpb"

# Create directories
mkdir -p "$MCPB_DIR"

# Platform configurations: GOOS GOARCH output_suffix platform_name
platforms=(
    "darwin arm64 darwin-arm64 darwin"
    "darwin amd64 darwin-amd64 darwin"
    "linux amd64 linux-amd64 linux"
    "linux arm64 linux-arm64 linux"
    "windows amd64 windows-amd64 win32"
)

echo "Building MCPB bundles for version $VERSION..."

for platform in "${platforms[@]}"; do
    read -r GOOS GOARCH suffix mcpb_platform <<< "$platform"

    binary_name="gleif-mcp-server"
    if [ "$GOOS" = "windows" ]; then
        binary_name="gleif-mcp-server.exe"
    fi

    bundle_dir="$MCPB_DIR/gleif-mcp-server-$suffix"
    bundle_file="$MCPB_DIR/gleif-mcp-server-$suffix.mcpb"

    echo "Building $GOOS/$GOARCH..."

    # Build binary
    CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-s -w" -o "$bundle_dir/$binary_name" .

    # Copy manifest with platform-specific adjustments
    jq --arg platform "$mcpb_platform" \
       '.compatibility.platforms = [$platform]' \
       manifest.json > "$bundle_dir/manifest.json"

    # Create .mcpb (zip archive)
    (cd "$bundle_dir" && zip -r "../gleif-mcp-server-$suffix.mcpb" .)

    # Calculate SHA-256
    shasum -a 256 "$bundle_file" | awk '{print $1}' > "$bundle_file.sha256"

    # Cleanup temp directory
    rm -rf "$bundle_dir"

    echo "  Created: $bundle_file"
done

echo ""
echo "MCPB bundles created in $MCPB_DIR/"
echo ""
echo "SHA-256 checksums:"
cat "$MCPB_DIR"/*.sha256

echo ""
echo "Next steps:"
echo "1. Upload .mcpb files to GitHub release v$VERSION"
echo "2. Create server.json pointing to the release URLs"
echo "3. Run: mcp-publisher publish"
