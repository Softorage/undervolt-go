#!/bin/bash
# update-undervolt.sh
# This script updates the undervolt Go binary in /usr/local/bin, to a new provided version
# making it available from anywhere in the terminal.
#
# Usage:
#   sudo ./update-undervolt.sh

set -e

# Ensure script is run as root.
if [[ $EUID -ne 0 ]]; then
  echo "This script must be run as root. Try using sudo."
  exit 1
fi

# Define paths
INSTALL_PATH="/usr/local/bin/undervolt-go-pro"
OLD_ICON_PATH="/usr/share/icons/undervolt-go.png"
ICON_PATH="/usr/share/pixmaps/undervolt-go.png"
WRAPPER_PATH="/usr/bin/undervolt-go-wrapper"

# Delete the existing binary if it exists.
if [ -f "$INSTALL_PATH" ]; then
  echo "Deleting existing binary at $INSTALL_PATH..."
  rm -f "$INSTALL_PATH"
fi

# Install the new binary
echo "Installing undervolt-go-pro to ${INSTALL_PATH}..."
cp undervolt-go-pro "${INSTALL_PATH}"
chmod +x "${INSTALL_PATH}"

# Update icon if present in the current directory
if [[ -f "icon.png" ]]; then
  # Delete the existing icon if it exists
  if [ -f "$ICON_PATH" ]; then
    echo "Deleting existing icon at $ICON_PATH..."
    rm -f "$ICON_PATH"
  fi
  # Delete icon if any at old invalid path
  if [ -f "$OLD_ICON_PATH" ]; then
    rm -f "$OLD_ICON_PATH"
  fi
  # Copy the new icon
  echo "Installing new icon to ${ICON_PATH}..."
  cp icon.png "${ICON_PATH}"
  chmod 644 "${ICON_PATH}"
else
  echo "No icon.png found in the current directory. Skipping icon update."
fi

# Reinstall desktop file
DESKTOP_FILE="/usr/share/applications/undervolt-go.desktop"
rm -f "${DESKTOP_FILE}"

echo "Creating desktop shortcut at ${DESKTOP_FILE}..."
cat <<EOF > "${DESKTOP_FILE}"
[Desktop Entry]
Name=Undervolt Go
Comment=Undervolt and tweak CPU power settings to reduce temperatures and improve performance
Exec=pkexec ${WRAPPER_PATH}
Icon=${ICON_PATH}
Terminal=false
Type=Application
Keywords=undervolt;throttlestop;cpu;
Categories=Utility;
EOF

chmod 644 "${DESKTOP_FILE}"

REAL_USER="${SUDO_USER:-$USER}"
USER_HOME=$(getent passwd "$REAL_USER" | cut -d: -f6)

# Determine Desktop folder (handling internationalization)
if [ "$EUID" -eq 0 ] && [ -n "$SUDO_USER" ]; then
    DESKTOP_FOLDER=$(sudo -u "$SUDO_USER" xdg-user-dir DESKTOP 2>/dev/null)
else
    DESKTOP_FOLDER=$(xdg-user-dir DESKTOP 2>/dev/null)
fi
# Fallback
if [ -z "$DESKTOP_FOLDER" ]; then
    DESKTOP_FOLDER="$USER_HOME/Desktop"
fi

# Get the .desktop on desktop
if [ -d "$DESKTOP_FOLDER" ]; then
    echo "Adding launcher to Desktop..."
    # Copy the already-modified desktop file from APP_DIR to the Desktop
    cp "${DESKTOP_FILE}" "$DESKTOP_FOLDER/"
    # Desktop files on the actual desktop MUST be executable to be "trusted"
    chmod +x "$DESKTOP_FOLDER/undervolt-go.desktop"
    # If running as root (via sudo), make sure the user owns the desktop file
    if [ "$EUID" -eq 0 ] && [ -n "$SUDO_USER" ]; then
        chown "$SUDO_USER:" "$DESKTOP_FOLDER/undervolt-go.desktop"
    fi
fi

echo "Update complete! You can now use the updated application from the terminal, applications menu or desktop. Try 'sudo undervolt-go-pro'."