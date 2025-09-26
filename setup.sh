#!/bin/bash

set -e

APP_NAME="simple-invoice"
BIN_NAME="${APP_NAME}-bin"
INSTALL_DIR="$HOME/.local/bin"
PROJECT_DIR="$HOME/.${APP_NAME}"

echo "Installing SimpleInvoice into $PROJECT_DIR..."
mkdir -p "$PROJECT_DIR"
cp -r . "$PROJECT_DIR"

cd "$PROJECT_DIR"
echo "Building Go binary..."
go build -o "$BIN_NAME" ./cmd/server

echo "Setting up Python environment..."
cd tools
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
cd ..

echo " Creating launcher script in $INSTALL_DIR/$APP_NAME..."
mkdir -p "$INSTALL_DIR"
cat <<EOF > "$INSTALL_DIR/$APP_NAME"
#!/bin/bash
cd "$PROJECT_DIR"
./$BIN_NAME "\$@"
EOF

chmod +x "$INSTALL_DIR/$APP_NAME"

# Warn if INSTALL_DIR is not in PATH
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
  echo "⚠️ $INSTALL_DIR is not in your PATH."
  echo "👉 Add this to your shell config (~/.bashrc or ~/.zshrc):"
  echo "export PATH=\"\$PATH:$INSTALL_DIR\""
fi

echo "✅ Done! You can now run simple-invoice with: $APP_NAME"

