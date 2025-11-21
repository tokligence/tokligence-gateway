#!/bin/bash
# Presidio Sidecar Setup Script
# This script sets up the Presidio firewall service in a virtual environment

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VENV_DIR="$SCRIPT_DIR/venv"

echo "=== Presidio Sidecar Setup ==="
echo "Installation directory: $SCRIPT_DIR"
echo ""

# Check Python version
if ! command -v python3 &> /dev/null; then
    echo "ERROR: python3 is not installed"
    echo "Please install Python 3.8+ first"
    exit 1
fi

PYTHON_VERSION=$(python3 --version | cut -d' ' -f2 | cut -d'.' -f1,2)
echo "Found Python version: $PYTHON_VERSION"

if [[ $(echo "$PYTHON_VERSION < 3.8" | bc -l) -eq 1 ]]; then
    echo "ERROR: Python 3.8+ is required (found $PYTHON_VERSION)"
    exit 1
fi

# Create virtual environment
if [ -d "$VENV_DIR" ]; then
    echo "Virtual environment already exists at: $VENV_DIR"
    read -p "Remove and recreate? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Removing old virtual environment..."
        rm -rf "$VENV_DIR"
    else
        echo "Using existing virtual environment"
    fi
fi

if [ ! -d "$VENV_DIR" ]; then
    echo "Creating virtual environment..."
    python3 -m venv "$VENV_DIR"
fi

# Activate virtual environment
echo "Activating virtual environment..."
source "$VENV_DIR/bin/activate"

# Upgrade pip
echo "Upgrading pip..."
pip install --upgrade pip

# Install requirements
echo "Installing Python dependencies..."
pip install -r "$SCRIPT_DIR/requirements.txt"

# Download spaCy model
echo "Downloading spaCy language model (this may take a few minutes)..."
python -m spacy download en_core_web_lg

echo ""
echo "=== Setup Complete ==="
echo ""
echo "To start the Presidio sidecar:"
echo "  cd $SCRIPT_DIR"
echo "  ./start.sh"
echo ""
echo "Or manually:"
echo "  source $VENV_DIR/bin/activate"
echo "  python main.py"
echo ""
