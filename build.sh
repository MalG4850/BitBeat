#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -e

# Target output directory
OUT_DIR="builds"
mkdir -p "$OUT_DIR"

echo "Building BitBeat for multiple platforms using vendored dependencies..."

# Array of targets in format "GOOS/GOARCH/OUTPUT_NAME"
TARGETS=(
    "linux/amd64/bitbeat-linux-amd64"
    "linux/arm64/bitbeat-linux-arm64"
    "windows/amd64/bitbeat-windows-amd64.exe"
    "darwin/amd64/bitbeat-darwin-amd64"
    "darwin/arm64/bitbeat-darwin-arm64"
)

for target in "${TARGETS[@]}"; do
    IFS="/" read -r goos goarch output_name <<< "$target"
    echo " -> Building for $goos/$goarch..."
    
    if [ "$goos" = "linux" ]; then
        # Linux oto driver requires CGO (ALSA development libraries)
        cgo=1
        if [ "$goarch" = "amd64" ]; then
            CGO_ENABLED=$cgo GOOS=$goos GOARCH=$goarch go build -mod=vendor -ldflags="-s -w" -trimpath -o "$OUT_DIR/$output_name" main.go
            # Copy to root as well
            cp "$OUT_DIR/$output_name" "$output_name"
        elif [ "$goarch" = "arm64" ]; then
            if command -v aarch64-linux-gnu-gcc >/dev/null 2>&1; then
                # Check for sysroot
                SYSROOT_DIR="arm64_sysroot"
                if [ ! -d "$SYSROOT_DIR" ]; then
                    echo " -> Downloading and extracting Debian arm64 ALSA sysroot for cross-compilation..."
                    mkdir -p "$SYSROOT_DIR"
                    curl -s -L -o "$SYSROOT_DIR/libasound.deb" http://ftp.debian.org/debian/pool/main/a/alsa-lib/libasound2t64_1.2.16-1_arm64.deb
                    curl -s -L -o "$SYSROOT_DIR/libasound-dev.deb" http://ftp.debian.org/debian/pool/main/a/alsa-lib/libasound2-dev_1.2.16-1_arm64.deb
                    
                    bsdtar -xf "$SYSROOT_DIR/libasound.deb" -C "$SYSROOT_DIR"
                    bsdtar -xf "$SYSROOT_DIR/data.tar.xz" -C "$SYSROOT_DIR"
                    rm -f "$SYSROOT_DIR/control.tar.xz" "$SYSROOT_DIR/data.tar.xz" "$SYSROOT_DIR/debian-binary"
                    
                    bsdtar -xf "$SYSROOT_DIR/libasound-dev.deb" -C "$SYSROOT_DIR"
                    bsdtar -xf "$SYSROOT_DIR/data.tar.xz" -C "$SYSROOT_DIR"
                    rm -f "$SYSROOT_DIR/control.tar.xz" "$SYSROOT_DIR/data.tar.xz" "$SYSROOT_DIR/debian-binary"
                    
                    rm -f "$SYSROOT_DIR/libasound.deb" "$SYSROOT_DIR/libasound-dev.deb"
                fi
                
                CC=aarch64-linux-gnu-gcc \
                CGO_CFLAGS="-I$(pwd)/$SYSROOT_DIR/usr/include" \
                CGO_LDFLAGS="-L$(pwd)/$SYSROOT_DIR/usr/lib/aarch64-linux-gnu" \
                CGO_ENABLED=$cgo GOOS=$goos GOARCH=$goarch \
                go build -mod=vendor -ldflags="-s -w" -trimpath -o "$OUT_DIR/$output_name" main.go
                # Copy to root as well
                cp "$OUT_DIR/$output_name" "$output_name"
            else
                echo "    [WARNING] Skipping linux/arm64: requires aarch64-linux-gnu-gcc for CGO build"
            fi
        else
            CGO_ENABLED=$cgo GOOS=$goos GOARCH=$goarch go build -mod=vendor -ldflags="-s -w" -trimpath -o "$OUT_DIR/$output_name" main.go
        fi
    else
        # Windows and macOS do not require CGO for oto v3
        cgo=0
        CGO_ENABLED=$cgo GOOS=$goos GOARCH=$goarch go build -mod=vendor -ldflags="-s -w" -trimpath -o "$OUT_DIR/$output_name" main.go
        # Copy to root as well
        cp "$OUT_DIR/$output_name" "$output_name"
    fi
done

echo "Build complete! Artifacts are located in the '$OUT_DIR' directory:"
ls -lh "$OUT_DIR"
