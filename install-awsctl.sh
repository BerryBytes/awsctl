#!/bin/bash

# Script to build, install, and configure the 'awsctl' binary

# Configuration
BINARY_NAME="awsctl"
BUILD_DIR="$(pwd)/bin"
INSTALL_DIR="$HOME/awsctl"
GC_FLAGS="all=-N -l"
LD_FLAGS="-s -w"

# Detect shell
detect_shell() {
  case "$SHELL" in
    */zsh)
      SHELL_CONFIG_FILE="$HOME/.zshrc"
      SHELL_NAME="zsh"
      ;;
    */bash)
      SHELL_CONFIG_FILE="$HOME/.bashrc"
      SHELL_NAME="bash"
      ;;
    *)
      error "Unsupported shell: $SHELL. Please use bash or zsh."
      ;;
  esac
  log "Detected shell: $SHELL_NAME"
}

# Logging function
log() {
  echo "[INFO] $1"
}

# Error handling function
error() {
  echo "[ERROR] $1" >&2
  exit 1
}

# Detect OS and architecture
detect_platform() {
  case "$(uname -s)" in
    Darwin)
      OS="darwin"
      ;;
    Linux)
      OS="linux"
      ;;
    *)
      error "Unsupported OS: $(uname -s)"
      ;;
  esac

  case "$(uname -m)" in
    x86_64)
      ARCH="amd64"
      ;;
    aarch64|arm64)
      ARCH="arm64"
      ;;
    *)
      error "Unsupported architecture: $(uname -m)"
      ;;
  esac

  log "Detected OS: $OS, Architecture: $ARCH"
}

# Build the Go binary
build_binary() {
  log "Building binary for OS: $OS, Architecture: $ARCH"
  GOOS="$OS" GOARCH="$ARCH" go build -gcflags="$GC_FLAGS" -ldflags="$LD_FLAGS" -o "$BUILD_DIR/$BINARY_NAME" ./main.go
  if [ $? -ne 0 ]; then
    error "Failed to build the binary."
  fi
  log "Binary built successfully at $BUILD_DIR/$BINARY_NAME"
}

# Move the binary to the target directory
install_binary() {
  log "Installing binary to $INSTALL_DIR"

  mkdir -p "$INSTALL_DIR" || error "Failed to create installation directory: $INSTALL_DIR"

  if [ -f "$INSTALL_DIR/$BINARY_NAME" ]; then
    log "Removing old binary..."
    rm -f "$INSTALL_DIR/$BINARY_NAME" || error "Failed to remove old binary."
  fi

  mv "$BUILD_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME" || error "Failed to move binary to $INSTALL_DIR."
  log "Binary installed successfully to $INSTALL_DIR/$BINARY_NAME"
}

# Make the binary executable
make_executable() {
  log "Making the binary executable..."
  chmod +x "$INSTALL_DIR/$BINARY_NAME" || error "Failed to make the binary executable."
}

# Add the installation directory to the user's PATH
update_path() {
  log "Updating $SHELL_CONFIG_FILE to include $INSTALL_DIR in PATH"

  if ! grep -q "$INSTALL_DIR" "$SHELL_CONFIG_FILE"; then
    echo "export PATH=\"$INSTALL_DIR:\$PATH\"" >> "$SHELL_CONFIG_FILE" || error "Failed to update $SHELL_CONFIG_FILE."
    log "Added $INSTALL_DIR to PATH in $SHELL_CONFIG_FILE."

    if [[ $- == *i* ]]; then
      log "Sourcing $SHELL_CONFIG_FILE to apply changes to the current shell..."
      source "$SHELL_CONFIG_FILE"
    else
      log "Please run 'source $SHELL_CONFIG_FILE' to apply changes to your current shell."
    fi
  else
    log "$INSTALL_DIR is already in PATH."
  fi
}

main() {
  detect_shell
  detect_platform
  build_binary
  install_binary
  make_executable
  update_path

  log "Installation complete! You can now run the '$BINARY_NAME' command."
}

# Execute the script
main
