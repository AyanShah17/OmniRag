#!/usr/bin/env bash
# OmniRAG Linux Post-Install Script
set -e

CONFIG_FILE="/etc/omnirag/omnirag.env"

echo "============================================================"
echo " OmniRAG Enterprise Backend Installed!"
echo "============================================================"
echo ""

if [ -t 0 ]; then
    echo "Launching Interactive Configuration Wizard..."
    /usr/bin/omnirag-config --config "$CONFIG_FILE"
else
    echo "Running in non-interactive mode."
    echo "Please run 'sudo omnirag-config' to set up your API keys."
fi

echo ""
echo "Enabling and starting OmniRAG systemd service..."
systemctl enable --now omnirag || true
echo "OmniRAG Service Status:"
systemctl status omnirag --no-pager || true
