package os

const (
	GreenbootFailScript = `#!/bin/bash

echo "greenboot detected a boot failure" >> /var/roothome/greenboot.log
date >> /var/roothome/greenboot.log
grub2-editenv list | grep boot_counter >> /var/roothome/greenboot.log
echo "----------------" >> /var/roothome/greenboot.log
echo "" >> /var/roothome/greenboot.log`


	GreenbootHealthCheckScript = `#!/bin/bash

#!/bin/bash

if [ -x /usr/libexec/yggdrasil/device-worker ]; then
  echo "device-worker found, check passed!"
else
  echo "device-worker not found, check failed!"
  exit 1
fi

check_is_service_active(){
  n=0
  until [[ "$n" -ge 5 ]]
  do
    if [[ $(systemctl is-active $1) = "active" ]]; then
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


check_is_service_active yggdrasild.service
check_is_service_active nftables.service
check_is_service_active podman.service
check_is_service_active podman.socket

exit 0`

)
