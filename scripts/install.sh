#!/bin/bash

# PDF-FTS Installation Script
# Downloads the latest release from GitHub and installs it to ~/.local/bin

set -e

# Colors and configuration
RED='\033[0;31m' GREEN='\033[0;32m' YELLOW='\033[1;33m' BLUE='\033[0;34m' NC='\033[0m'
REPO="aziis98/pdf-fts"
BINARY_NAME="pdf-fts"
INSTALL_DIR="${INSTALL_DIR_OVERRIDE:-$HOME/.local/bin}"

# Print colored status messages
print_status() { echo -e "${1}[${2}]${NC} $3"; }

# Detect platform and return binary name
detect_platform() {
    local os arch suffix=""
    
    case "$(uname -s)" in
        Linux*) os="Linux" ;;
        Darwin*) os="Darwin" ;;
        CYGWIN*|MINGW*|MSYS*) os="Windows"; suffix=".exe" ;;
        *) print_status "$RED" "ERROR" "Unsupported OS: $(uname -s)"; exit 1 ;;
    esac
    
    case "$(uname -m)" in
        x86_64|amd64) arch="x86_64" ;;
        arm64|aarch64) arch="arm64" ;;
        *) print_status "$RED" "ERROR" "Unsupported architecture: $(uname -m)"; exit 1 ;;
    esac
    
    echo "${BINARY_NAME}-${os}-${arch}${suffix}"
}

# Download and install binary
install_binary() {
    local binary_name="$1"
    local url="https://github.com/${REPO}/releases/latest/download/${binary_name}"
    local temp_file="/tmp/${binary_name}"
    local install_path="${INSTALL_DIR}/${BINARY_NAME}"
    
    print_status "$BLUE" "INFO" "Downloading ${binary_name}..."
    curl -sSL "$url" -o "$temp_file" || {
        print_status "$RED" "ERROR" "Download failed. Check: https://github.com/${REPO}/releases"
        exit 1
    }
    
    print_status "$BLUE" "INFO" "Installing to ${install_path}..."
    mv "$temp_file" "$install_path" && chmod +x "$install_path" || {
        print_status "$RED" "ERROR" "Installation failed"
        exit 1
    }
    
    print_status "$GREEN" "SUCCESS" "Installation complete!"
}

# Verify installation and check PATH
verify_installation() {
    local install_path="${INSTALL_DIR}/${BINARY_NAME}"
    local version=$("$install_path" --version 2>/dev/null || echo "installed")
    
    print_status "$GREEN" "SUCCESS" "pdf-fts ready! Version: $version"
    
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        print_status "$YELLOW" "WARNING" "$INSTALL_DIR not in PATH"
        print_status "$BLUE" "INFO" "Add to PATH: echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.bashrc && source ~/.bashrc"
        print_status "$BLUE" "INFO" "Or run directly: $install_path"
    else
        print_status "$BLUE" "INFO" "Run 'pdf-fts --help' to get started"
    fi
}

# Main installation
main() {
    print_status "$BLUE" "INFO" "PDF-FTS Installation Script"
    
    # Check dependencies and setup
    command -v curl >/dev/null || { print_status "$RED" "ERROR" "curl required"; exit 1; }
    [ "$EUID" -eq 0 ] && print_status "$YELLOW" "WARNING" "Running as root"
    
    print_status "$BLUE" "INFO" "Installing to: $INSTALL_DIR"
    mkdir -p "$INSTALL_DIR" || { print_status "$RED" "ERROR" "Cannot create $INSTALL_DIR"; exit 1; }
    
    # Detect, download, install, verify
    local binary_name=$(detect_platform)
    print_status "$BLUE" "INFO" "Platform: $binary_name"
    
    install_binary "$binary_name"
    verify_installation
    
    print_status "$GREEN" "SUCCESS" "Installation completed!"
}

# Run main function
main "$@"
