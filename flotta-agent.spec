%define _build_id_links none

Name:       flotta-agent
Version:    1.0
Release:    1%{?dist}
Summary:    Agent application for the Flotta Edge Management solution
ExclusiveArch: %{go_arches}
Group:      Flotta
License:    GPL
Source0:    https://github.com/jakub-dzon/%{name}-%{version}.tar.gz

BuildRequires:  golang

Requires:       nftables
Requires:       podman
Requires:       yggdrasil

Provides:       %{name} = %{version}-%{release}
Provides:       golang(%{go_import_path}) = %{version}-%{release}

%description
The Flotta agent communicates with the Flotta control plane. It reports the status of the appliance and of the running PODs/containers. Agent is responsible for starting and stopping PODs that are based on commands from the control plane.

%post
systemctl enable --now podman.socket
systemctl enable --now nftables.service

%prep 
tar fx %{SOURCE0}

%build
cd flotta-agent-%{VERSION}
export CGO_ENABLED=0 
export GOFLAGS="-mod=vendor -tags=containers_image_openpgp"
go build -o ./bin/device-worker ./cmd/device-worker

%install
cd flotta-agent-%{VERSION}
mkdir -p %{buildroot}%{_libexecdir}/yggdrasil/
install ./bin/device-worker %{buildroot}%{_libexecdir}/yggdrasil/device-worker
make install-worker-config LIBEXECDIR=%{_libexecdir} BUILDROOT=%{buildroot} SYSCONFDIR=%{_sysconfdir}

%files
%{_libexecdir}/yggdrasil/device-worker
%{_sysconfdir}/yggdrasil/

%changelog

* Fri Feb 11 2022 Eloy Coto <eloycoto@acalustra.com> 1.1
Using latest version of yggdrasil

* Tue Nov 02 2021 Eloy Coto <eloycoto@acalustra.com> 1.0
Compile Go code directly on the RPM spec, don't ship binaries.

* Thu Sep 23 2021 Piotr Kliczewski - 1.0
- TBD
