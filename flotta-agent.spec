%define _build_id_links none
%global flotta_systemd packaging/systemd
%global flotta_user flotta

Name:       flotta-agent
Version:    0.1.0
Release:    2%{?dist}
Summary:    Agent application for the Flotta Edge Management solution
ExclusiveArch: %{go_arches}
Group:      Flotta
License:    ASL 2.0
Source0:    %{name}-%{version}.tar.gz

BuildRequires:  golang
BuildRequires:  systemd-rpm-macros

%if 0%{?rhel}
Requires:       ansible-core
%else
Requires:       ansible
%endif
Requires:       nftables
Requires:       node_exporter
Requires:       podman
Requires:       yggdrasil

Requires(pre): shadow-utils

Provides:       %{name} = %{version}-%{release}
Provides:       golang(%{go_import_path}) = %{version}-%{release}

%package        race
Summary:        flotta-agent with race-enabled

%description race
The same as flotta agent, but in this case compiled with --race flag to be able
to detect race-conditions in the e2e test

%description
The Flotta agent communicates with the Flotta control plane. It reports the status of the appliance and of the running PODs/containers. Agent is responsible for starting and stopping PODs that are based on commands from the control plane.

%prep
tar fx %{SOURCE0}
%sysusers_create_compat flotta-agent-%{VERSION}/%{flotta_systemd}/flotta-agent.sysusers

%build
cd flotta-agent-%{VERSION}
export CGO_ENABLED=0
export GOFLAGS="-mod=vendor -tags=containers_image_openpgp"
go build -o ./bin/device-worker ./cmd/device-worker
go build -o ./bin/device-worker-race ./cmd/device-worker

%install
cd flotta-agent-%{VERSION}
mkdir -p %{buildroot}%{_libexecdir}/yggdrasil/
install ./bin/device-worker %{buildroot}%{_libexecdir}/yggdrasil/device-worker
install ./bin/device-worker-race %{buildroot}%{_libexecdir}/yggdrasil/device-worker-race
make install-worker-config USER=%{flotta_user} HOME=/var/home/%{flotta_user} LIBEXECDIR=%{_libexecdir} BUILDROOT=%{buildroot} SYSCONFDIR=%{_sysconfdir}

install -Dpm 644 %{flotta_systemd}/flotta-agent.sysusers %{buildroot}%{_sysusersdir}/%{name}.conf
install -Dpm 644 %{flotta_systemd}/flotta.conf %{buildroot}/etc/tmpfiles.d/%{name}.conf

%files
%{_libexecdir}/yggdrasil/device-worker
%{_sysconfdir}/yggdrasil/
%{_sysusersdir}/%{name}.conf
/etc/tmpfiles.d/%{name}.conf

%files race
%{_libexecdir}/yggdrasil/device-worker-race
%{_sysconfdir}/yggdrasil/
%{_sysusersdir}/%{name}.conf
/etc/tmpfiles.d/%{name}.conf

%post
systemctl enable --now nftables.service

%post race
ln -sf %{_libexecdir}/yggdrasil/device-worker-race %{_libexecdir}/yggdrasil/device-worker
systemctl enable --now nftables.service

%changelog
* Wed Jun 22 2022 Eloy Coto <eloycoto@acalustra.com> 0.1.0-2
  Changes on systemd config

* Thu May 12 2022 Ondra Machacek <omachace@redhat.com> 0.1.0-1
- Initial release.
