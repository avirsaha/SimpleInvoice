#!/bin/bash

set -e

APP_NAME="simple_invoice"
INSTALL_DIR="$HOME/.local/bin"

echo "Building Go binary..."
go build -o $APP_NAME ./cmd/server
echo "Binary '$APP_NAME' built successfully."

# Ensure ~/.local/bin exists
mkdir -p "$INSTALL_DIR"

# Move the binary to ~/.local/bin
echo "Installing binary to $INSTALL_DIR..."
mv $APP_NAME "$INSTALL_DIR/"

# Make sure ~/.local/bin is in PATH
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
  echo "Warning: $INSTALL_DIR is not in your PATH."
  echo "You can add it by adding this line to your shell profile (e.g., ~/.bashrc or ~/.zshrc):"
  echo "export PATH=\"\$PATH:$INSTALL_DIR\""
else
  echo "You can now run the program with: $APP_NAME"
fi

# Python tool setup (optional)
echo "Setting up Python environment..."
cd tools

if [ ! -d "venv" ]; then
  python3 -m venv venv
fi

source venv/bin/activate

pip install --upgrade pip
pip install -r requirements.txt

cd ..

echo "Setup complete!"

