%define _build_id_links none
%global flotta_systemd packaging/systemd
%global flotta_user flotta

Name:       flotta-agent
Version:    0.2.0
Release:    2%{?dist}
Summary:    Agent application for the Flotta Edge Management solution
ExclusiveArch: %{go_arches}
Group:      Flotta
License:    ASL 2.0
Source0:    %{name}-%{version}.tar.gz

BuildRequires:  golang
BuildRequires:  systemd-rpm-macros
BuildRequires:  bash
%if 0%{?fedora} && ! 0%{?rhel}
BuildRequires: btrfs-progs-devel
%endif
BuildRequires:  device-mapper-devel

%if 0%{?rhel}
Requires:       ansible-core
%else
Requires:       ansible
%endif
Requires:       nftables
Requires:       node_exporter
Requires:       podman >= 4.2.0
Requires:       yggdrasil

Requires(pre): shadow-utils

Provides:       %{name} = %{version}-%{release}
Provides:       golang(%{go_import_path}) = %{version}-%{release}

%package        race
Summary:        flotta-agent with race-enabled

%description race
The same as flotta agent, but in this case compiled with -race flag to be able
to detect race-conditions in the e2e test

%description
The Flotta agent communicates with the Flotta control plane. It reports the status of the appliance and of the running PODs/containers. Agent is responsible for starting and stopping PODs that are based on commands from the control plane.

%prep
tar fx %{SOURCE0}

# RHEL does not support btrfs
# https://github.com/containers/podman/blob/948c5e915aec709beb4e171a72c7e54504889baf/podman.spec.rpkg#L162-L164
%if 0%{?rhel}
rm -rf flotta-agent-%{VERSION}/vendor/github.com/containers/storage/drivers/register/register_btrfs.go
%endif

%build
cd flotta-agent-%{VERSION}
export CGO_ENABLED=0
export GOFLAGS="-mod=vendor -tags=containers_image_openpgp"
go build -o ./bin/device-worker ./cmd/device-worker
export CGO_ENABLED=1
go build -race -o ./bin/device-worker-race ./cmd/device-worker

%install
cd flotta-agent-%{VERSION}
mkdir -p %{buildroot}%{_libexecdir}/yggdrasil/
install ./bin/device-worker %{buildroot}%{_libexecdir}/yggdrasil/device-worker
install ./bin/device-worker-race %{buildroot}%{_libexecdir}/yggdrasil/device-worker-race
make install-worker-config USER=%{flotta_user} HOME=/var/home/%{flotta_user} LIBEXECDIR=%{_libexecdir} BUILDROOT=%{buildroot} SYSCONFDIR=%{_sysconfdir}

install -Dpm 644 %{flotta_systemd}/flotta-agent.service %{buildroot}%{_unitdir}/%{name}.service
install -Dpm 644 %{flotta_systemd}/flotta.conf %{buildroot}/etc/tmpfiles.d/%{name}.conf

%files
%{_libexecdir}/yggdrasil/device-worker
%{_sysconfdir}/yggdrasil/
/etc/tmpfiles.d/%{name}.conf
%{_unitdir}/%{name}.service

%files race
%{_libexecdir}/yggdrasil/device-worker-race
%{_sysconfdir}/yggdrasil/
%{_unitdir}/%{name}.service
/etc/tmpfiles.d/%{name}.conf

%post
systemctl enable --now nftables.service
ln -s -f %{_unitdir}/%{name}.service /etc/systemd/system/multi-user.target.wants/flotta-agent.service
systemctl start flotta-agent || exit 0 # can fail on rpm-ostree base system

%post race
ln -sf %{_libexecdir}/yggdrasil/device-worker-race %{_libexecdir}/yggdrasil/device-worker
systemctl enable --now nftables.service
ln -s -f %{_unitdir}/%{name}.service /etc/systemd/system/multi-user.target.wants/flotta-agent.service
systemctl start flotta-agent || exit 0 # can fail on rpm-ostree base system

%changelog
* Tue Jul 26 2022 Moti Asayag <masayag@redhat.com> 0.2.0-2
- Start containers by systemd to be owned by flotta user
- Propogate annotations to to podman's kube struct

* Thu Jul 14 2022 Moti Asayag <masayag@redhat.com> 0.2.0-1
- Added support for rootless podman
- Added support for host devices

* Thu Jun 23 2022 Jordi Gil <jgil@redhat.com> 0.1.0-3
  Added missing '-race' to go build command for race package

* Wed Jun 22 2022 Eloy Coto <eloycoto@acalustra.com> 0.1.0-2
  Changes on systemd config

* Thu May 12 2022 Ondra Machacek <omachace@redhat.com> 0.1.0-1
- Initial release.
