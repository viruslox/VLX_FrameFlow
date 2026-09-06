#!/bin/bash
set -e

echo "Cleaning FrameFlow build artifacts..."
rm -rf build
rm -rf frontend_app/node_modules
rm -rf frontend_app/dist
rm -rf internal/ui/dist
echo "Clean complete."
