#!/bin/bash
# Quick start script for PateProject
# Usage: ./start.sh

# Load nvm
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"

# Use Node 22
nvm use 22 2>/dev/null || nvm install 22

# Run dev script
./dev.sh
