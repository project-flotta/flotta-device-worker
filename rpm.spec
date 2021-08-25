%define _build_id_links none

Summary: Agent application for the K4E (Kubernetes for Edge) solution
License: GPL
Name: k4e-agent
Version: %{VERSION}
Release: %{RELEASE}
Group: K4E
Requires: podman nftables

%description
The K4E agent communicates with the K4E control plane. It reports the status of the appliance and of the running PODs/containers. Agent is responsible for starting and stopping PODs that are based on commands from the control plane.

%files
%{_libexecdir}/yggdrasil/device-worker

%post
systemctl enable --now podman.socket
systemctl enable --now nftables.service
