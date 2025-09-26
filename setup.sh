#!/bin/bash

set -e  # exit on any error

echo "Building Go server..."
go build -o server ./cmd/server
echo "Go server built successfully."

echo "Setting up Python environment for tools..."
cd tools

if [ ! -d "venv" ]; then
  echo "Creating Python virtual environment..."
  python3 -m venv venv
fi

source venv/bin/activate

echo "Installing Python dependencies..."
pip install --upgrade pip
pip install -r requirements.txt

echo "Python environment setup complete."

cd ..

echo "Setup completed successfully!"

