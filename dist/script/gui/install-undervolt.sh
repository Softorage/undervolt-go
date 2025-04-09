#!/bin/bash
# install-undervolt.sh
# Installs undervolt-go and sets up desktop launcher with pkexec

set -e

# Ensure script is run as root
if [[ $EUID -ne 0 ]]; then
  echo "This script must be run as root. Try using sudo."
  exit 1
fi

echo "Installing Undervolt Go..."

# Install binary
INSTALL_PATH="/usr/local/bin/undervolt-go-pro"
cp undervolt-go-pro "${INSTALL_PATH}"
chmod +x "${INSTALL_PATH}"

# Install wrapper
WRAPPER_PATH="/usr/bin/undervolt-go-wrapper"
echo "Creating pkexec wrapper at ${WRAPPER_PATH}..."
cat <<EOF > "${WRAPPER_PATH}"
#!/bin/bash
export DISPLAY=:0
export XAUTHORITY=/home/\${SUDO_USER}/.Xauthority
exec ${INSTALL_PATH}
EOF
chmod +x "${WRAPPER_PATH}"

# Install icon (optional: replace with your own icon)
ICON_PATH="/usr/share/icons/undervolt-go.png"
if [[ -f "icon.png" ]]; then
  cp icon.png "${ICON_PATH}"
else
  echo "No icon.png found. You can place your own icon at ${ICON_PATH} later."
fi

# Install PolicyKit file
POLKIT_FILE="/usr/share/polkit-1/actions/com.softorage.undervolt-go.policy"
echo "Creating PolicyKit policy at ${POLKIT_FILE}..."
cat <<EOF > "${POLKIT_FILE}"
<?xml version="1.0" encoding="UTF-8"?>
<policyconfig>
  <action id="com.softorage.undervolt-go">
    <description>Run 'Undervolt Go' as root</description>
    <message>Authentication is required to run 'Undervolt Go'</message>
    <icon_name>utilities-system-monitor</icon_name>
    <defaults>
      <allow_any>auth_admin</allow_any>
      <allow_inactive>auth_admin</allow_inactive>
      <allow_active>auth_admin</allow_active>
    </defaults>
  </action>
</policyconfig>
EOF

# Install desktop file
DESKTOP_FILE="/usr/share/applications/undervolt-go.desktop"
echo "Creating desktop shortcut at ${DESKTOP_FILE}..."
cat <<EOF > "${DESKTOP_FILE}"
[Desktop Entry]
Name=Undervolt Go
Comment=Undervolt and tweak CPU power settings to reduce temperatures and improve performance
Exec=pkexec ${WRAPPER_PATH}
Icon=${ICON_PATH}
Terminal=false
Type=Application
Categories=Utility;
EOF

chmod +x "${DESKTOP_FILE}"

# Copy to user's desktop
USER_DESKTOP="${SUDO_USER:-$USER}"
USER_HOME=$(eval echo "~${USER_DESKTOP}")
cp "${DESKTOP_FILE}" "${USER_HOME}/Desktop/"
chmod +x "${USER_HOME}/Desktop/undervolt-go.desktop"
chown "${USER_DESKTOP}:${USER_DESKTOP}" "${USER_HOME}/Desktop/undervolt-go.desktop"

echo "Installation complete!"
echo "You can now launch 'Undervolt Go' from the applications menu or desktop."