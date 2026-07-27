#!/usr/bin/env bash
set -euo pipefail

RELAY_USER="videowithyou"
RELAY_PORT="21314"
SSHD_DROPIN="/etc/ssh/sshd_config.d/90-videowithyou.conf"

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root: sudo bash $0 /path/to/id_ed25519.pub" >&2
  exit 1
fi
if [[ $# -ne 1 || ! -f "$1" ]]; then
  echo "Usage: sudo bash $0 /path/to/id_ed25519.pub" >&2
  exit 1
fi

PUBLIC_KEY="$(tr -d '\r\n' < "$1")"
if [[ "${PUBLIC_KEY}" != ssh-ed25519\ * ]]; then
  echo "The supplied file is not an ssh-ed25519 public key." >&2
  exit 1
fi

if ! id "${RELAY_USER}" >/dev/null 2>&1; then
  useradd --create-home --shell /bin/bash "${RELAY_USER}"
fi
passwd -l "${RELAY_USER}" >/dev/null 2>&1 || true

USER_HOME="$(getent passwd "${RELAY_USER}" | cut -d: -f6)"
install -d -m 700 -o "${RELAY_USER}" -g "${RELAY_USER}" "${USER_HOME}/.ssh"
AUTHORIZED_LINE="restrict,port-forwarding,permitlisten=\"0.0.0.0:${RELAY_PORT}\" ${PUBLIC_KEY}"
printf '%s\n' "${AUTHORIZED_LINE}" > "${USER_HOME}/.ssh/authorized_keys"
chown "${RELAY_USER}:${RELAY_USER}" "${USER_HOME}/.ssh/authorized_keys"
chmod 600 "${USER_HOME}/.ssh/authorized_keys"

install -d -m 755 /etc/ssh/sshd_config.d
cat > "${SSHD_DROPIN}" <<EOF
Match User ${RELAY_USER}
    PasswordAuthentication no
    KbdInteractiveAuthentication no
    AllowAgentForwarding no
    AllowTcpForwarding remote
    GatewayPorts clientspecified
    PermitListen 0.0.0.0:${RELAY_PORT}
    PermitTTY no
    X11Forwarding no
EOF

if ! grep -Eq '^[[:space:]]*Include[[:space:]]+/etc/ssh/sshd_config\.d/\*\.conf' /etc/ssh/sshd_config; then
  echo "Your sshd_config does not include /etc/ssh/sshd_config.d/*.conf." >&2
  echo "Add this line near the top, then rerun this script:" >&2
  echo "Include /etc/ssh/sshd_config.d/*.conf" >&2
  exit 1
fi

sshd -t
if systemctl list-unit-files ssh.service >/dev/null 2>&1; then
  systemctl reload ssh
else
  systemctl reload sshd
fi

if command -v ufw >/dev/null 2>&1 && ufw status | grep -q '^Status: active'; then
  ufw allow "${RELAY_PORT}/tcp"
fi

echo "Cloud IPv4 relay is ready on TCP ${RELAY_PORT}."
echo "Also allow TCP ${RELAY_PORT} in the cloud provider security group."
