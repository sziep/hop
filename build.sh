#!/usr/bin/env bash
set -euo pipefail

OUTPUT_DIR="bin"
VERSION="${VERSION:-0.2.0}"
TARGETS="darwin/arm64 darwin/amd64 linux/arm64 linux/amd64"

mkdir -p "$OUTPUT_DIR"

echo "Building hop v$VERSION via Docker for: $TARGETS"

docker run --rm \
  -v "$(pwd)":/src \
  -w /src \
  -e CGO_ENABLED=0 \
  golang:alpine \
  sh -c "
    set -e
    go mod tidy
    for target in $TARGETS; do
      echo \"  \$target\"
      GOOS=\${target%/*} GOARCH=\${target#*/} \
        go build -ldflags='-s -w -X main.version=$VERSION' -trimpath \
        -o $OUTPUT_DIR/hop-\${target%/*}-\${target#*/} .
    done
  "

# convenience copy for the host platform
HOST_OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
HOST_ARCH="$(uname -m)"
case "$HOST_ARCH" in
  x86_64) HOST_ARCH=amd64 ;;
  aarch64) HOST_ARCH=arm64 ;;
esac
if [[ -f "$OUTPUT_DIR/hop-$HOST_OS-$HOST_ARCH" ]]; then
  # rm first: overwriting an executable in place trips macOS's cached
  # code-signature check and the binary gets SIGKILLed on launch
  rm -f "$OUTPUT_DIR/hop"
  cp "$OUTPUT_DIR/hop-$HOST_OS-$HOST_ARCH" "$OUTPUT_DIR/hop"
fi

echo "Done: ./$OUTPUT_DIR/hop (native) plus cross builds in ./$OUTPUT_DIR/"
echo ""
echo "Install steps:"
echo "  1. cp bin/hop /usr/local/bin/hop"
echo "  2. Add to ~/.zshrc:  eval \"\$(hop init zsh)\""
echo "     (bash: eval \"\$(hop init bash)\" · fish: hop init fish | source)"
echo "  3. Reload your shell"
