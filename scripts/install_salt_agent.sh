#!/bin/bash
set -euo pipefail

SALT_AGENT_HOME="${SALT_AGENT_HOME:-/data/salt-agent}"
PROFILE_FILE="/etc/profile.d/salt-agent.sh"

mkdir -p "${SALT_AGENT_HOME}" "${SALT_AGENT_HOME}/scripts"

profile_content="export SALT_AGENT_HOME=${SALT_AGENT_HOME}
export PATH=\$PATH:\$SALT_AGENT_HOME:\$SALT_AGENT_HOME/scripts"

if command -v sudo >/dev/null 2>&1; then
  printf '%s\n' "${profile_content}" | sudo tee "${PROFILE_FILE}" >/dev/null
else
  printf '%s\n' "${profile_content}" > "${PROFILE_FILE}"
fi

chmod -R a+rx "${SALT_AGENT_HOME}/scripts" 2>/dev/null || true

echo "Salt-Agent environment configured successfully."
echo "SALT_AGENT_HOME=${SALT_AGENT_HOME}"
echo "Profile file: ${PROFILE_FILE}"
