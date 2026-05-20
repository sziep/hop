#!/usr/bin/env bash
set -euo pipefail

BINARY_NAME="hop"
OUTPUT_DIR="bin"

mkdir -p "$OUTPUT_DIR"

echo "Building $BINARY_NAME for darwin/arm64 via Docker..."

docker run --rm \
  -v "$(pwd)":/src \
  -w /src \
  -e CGO_ENABLED=0 \
  -e GOOS=darwin \
  -e GOARCH=arm64 \
  golang:alpine \
  sh -c "go mod tidy && go build -ldflags='-s -w' -trimpath -o $OUTPUT_DIR/$BINARY_NAME ."

echo "Done: ./$OUTPUT_DIR/$BINARY_NAME"
echo ""
echo "Install steps:"
echo "  1. Copy binary somewhere on your PATH, e.g.:"
echo "       cp bin/hop /usr/local/bin/hop"
echo "  2. Add to ~/.zshrc:"
echo "       eval \"\$(hop init zsh)\""
echo "  3. Reload: source ~/.zshrc"
echo "  4. Optional alias: alias cv=hop"
