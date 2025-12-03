#!/bin/bash

PROFILE_FILE=$(find $HOME -name '.frameflow_profile' 2>/dev/null)
[ -f "$PROFILE_FILE" ] && source "$PROFILE_FILE"

VLXlogs_DIR="${VLXlogs_DIR:-/opt/VLXflowlogs}"

# Clean old logs
find "$VLXlogs_DIR" -type f -mtime +15 -exec rm -v {} \;
find /var/log -type f -mtime +30 -exec rm -v {} \;

# Backup installed packages list
dpkg --get-selections | awk '{print $1}' | \
grep -vE '^(linux-image|linux-headers|firmware|grub|nvidia|base-files)' > "/root/pkg.list"

exit 0
