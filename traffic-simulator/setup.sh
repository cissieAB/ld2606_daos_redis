#!/bin/bash
# Setup script for Traffic Simulator

set -e

echo "=== Traffic Simulator Setup ==="
echo

# Check if Python 3 is available
if ! command -v python3 &> /dev/null; then
    echo "Error: Python 3 is not installed"
    echo "Please install Python 3 and try again"
    exit 1
fi

echo "Python version: $(python3 --version)"
echo

# Create virtual environment if it doesn't exist
if [ ! -d "venv" ]; then
    echo "Creating virtual environment..."
    python3 -m venv venv
    echo "✓ Virtual environment created"
else
    echo "✓ Virtual environment already exists"
fi

echo

# Activate virtual environment and install dependencies
echo "Installing dependencies..."
source venv/bin/activate
pip install --upgrade pip -q
pip install -r requirements.txt -q
echo "✓ Dependencies installed"

echo
echo "=== Setup Complete ==="
echo
echo "To use the simulator:"
echo "  1. Activate the environment: source venv/bin/activate"
echo "  2. Run the simulator: python simulator.py --redis-host <host>"
echo
echo "Example:"
echo "  source venv/bin/activate"
echo "  python3 simulator.py --redis-host ejfat-6.jlab.org"
echo
echo "For more options, see: python3 simulator.py --help"
echo "Or read: README.md"
