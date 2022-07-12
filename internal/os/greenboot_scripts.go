package os

const (
	GreenbootFailScript = `#!/bin/bash

echo "greenboot detected a boot failure" >> /var/roothome/greenboot.log
date >> /var/roothome/greenboot.log
grub2-editenv list | grep boot_counter >> /var/roothome/greenboot.log
echo "----------------" >> /var/roothome/greenboot.log
echo "" >> /var/roothome/greenboot.log`

	GreenbootHealthCheckScript = `
#!/bin/bash
if [ -x /usr/libexec/yggdrasil/device-worker ]; then
  echo "device-worker found, check passed!"
else
  echo "device-worker not found, check failed!"
  exit 1
fi

check_is_service_active(){
  local service=$1
  local sudo_params=$2
  local systemctl_params=$3
  n=0
  until [[ "$n" -ge 5 ]]
  do
    if [[ $(sudo $sudo_params systemctl is-active $systemctl_params $service) = "active" ]]; then
      echo "$1 is active, check passed!"
      return
    else
      echo "$1 is not active, retrying"
    fi
    n=$((n+1))
    sleep 1
  done
  echo "service $1 is not active"
  exit 1
}

params="-u flotta DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/$(id -u flotta)/bus"

check_is_service_active yggdrasild.service
check_is_service_active nftables.service
check_is_service_active podman.service "$params" "--user"
check_is_service_active podman.socket "$params" "--user"

exit 0`
)
