%if 0%{?fedora} || 0%{?rhel} == 6
%global with_devel 0
%global with_bundled 1
%global with_debug 1
%global with_check 0
%global with_unit_test 0
%else
%global with_devel 0
%global with_bundled 1
%global with_debug 1
%global with_check 0
%global with_unit_test 0
%endif

#%%if 0%%{?with_debug}
#%%global _dwz_low_mem_die_limit 0
#%%else
%global debug_package   %{nil}
#%%endif

%global provider        github
%global provider_tld    com
%global project         projectatomic
%global repo            skopeo
# https://github.com/projectatomic/skopeo
%global provider_prefix %{provider}.%{provider_tld}/%{project}/%{repo}
%global import_path     %{provider_prefix}
%global git0            https://%{import_path}
%global commit0         REPLACEWITHCOMMITID
%global shortcommit0    %(c=%{commit0}; echo ${c:0:7})

# e.g. el6 has ppc64 arch without gcc-go, so EA tag is required
# manually listed arches due https://bugzilla.redhat.com/show_bug.cgi?id=1391932 (removed ppc64)
ExcludeArch: ppc64

Name:           %{repo}
%if 0%{?centos}
Epoch:          1
%endif # centos
Version:        0.1.27
Release:        1.git%{shortcommit0}%{?dist}
Summary:        Inspect Docker images and repositories on registries
License:        ASL 2.0
URL:            %{git0}
Source0:        %{git0}/archive/%{commit0}/%{name}-%{shortcommit0}.tar.gz
Source1:        storage.conf
Source2:        containers-storage.conf.5.md
Source3:        mounts.conf

%if 0%{?fedora}
BuildRequires: go-srpm-macros
BuildRequires: compiler(go-compiler)
%endif
BuildRequires:  git
BuildRequires:  make
# If go_compiler is not set to 1, there is no virtual provide. Use golang instead.
BuildRequires:  %{?go_compiler:compiler(go-compiler)}%{!?go_compiler:golang}
BuildRequires:  golang-github-cpuguy83-go-md2man
BuildRequires:  gpgme-devel
BuildRequires:  libassuan-devel
# Dependencies for containers/storage
BuildRequires:  btrfs-progs-devel
BuildRequires:  pkgconfig(devmapper)
BuildRequires:  ostree-devel
BuildRequires:  glib2-devel

Requires: containers-common = %{version}-%{release}

%description
Command line utility to inspect images and repositories directly on Docker
registries without the need to pull them

%if 0%{?with_devel}
%package devel
Summary:       %{summary}
BuildArch:     noarch

%if 0%{?with_check} && ! 0%{?with_bundled}
BuildRequires: golang(github.com/Azure/go-ansiterm/winterm)
BuildRequires: golang(github.com/Sirupsen/logrus)
BuildRequires: golang(github.com/docker/distribution)
BuildRequires: golang(github.com/docker/distribution/context)
BuildRequires: golang(github.com/docker/distribution/digest)
BuildRequires: golang(github.com/docker/distribution/manifest)
BuildRequires: golang(github.com/docker/distribution/manifest/manifestlist)
BuildRequires: golang(github.com/docker/distribution/manifest/schema1)
BuildRequires: golang(github.com/docker/distribution/manifest/schema2)
BuildRequires: golang(github.com/docker/distribution/reference)
BuildRequires: golang(github.com/docker/distribution/registry/api/errcode)
BuildRequires: golang(github.com/docker/distribution/registry/api/v2)
BuildRequires: golang(github.com/docker/distribution/registry/client)
BuildRequires: golang(github.com/docker/distribution/registry/client/auth)
BuildRequires: golang(github.com/docker/distribution/registry/client/transport)
BuildRequires: golang(github.com/docker/distribution/registry/storage/cache)
BuildRequires: golang(github.com/docker/distribution/registry/storage/cache/memory)
BuildRequires: golang(github.com/docker/distribution/uuid)
BuildRequires: golang(github.com/docker/docker/api)
BuildRequires: golang(github.com/docker/docker/daemon/graphdriver)
BuildRequires: golang(github.com/docker/docker/distribution/metadata)
BuildRequires: golang(github.com/docker/docker/distribution/xfer)
BuildRequires: golang(github.com/docker/docker/dockerversion)
BuildRequires: golang(github.com/docker/docker/image)
BuildRequires: golang(github.com/docker/docker/image/v1)
BuildRequires: golang(github.com/docker/docker/layer)
BuildRequires: golang(github.com/docker/docker/opts)
BuildRequires: golang(github.com/docker/docker/pkg/archive)
BuildRequires: golang(github.com/docker/docker/pkg/chrootarchive)
BuildRequires: golang(github.com/docker/docker/pkg/fileutils)
BuildRequires: golang(github.com/docker/docker/pkg/homedir)
BuildRequires: golang(github.com/docker/docker/pkg/httputils)
BuildRequires: golang(github.com/docker/docker/pkg/idtools)
BuildRequires: golang(github.com/docker/docker/pkg/ioutils)
BuildRequires: golang(github.com/docker/docker/pkg/jsonlog)
BuildRequires: golang(github.com/docker/docker/pkg/jsonmessage)
BuildRequires: golang(github.com/docker/docker/pkg/longpath)
BuildRequires: golang(github.com/docker/docker/pkg/mflag)
BuildRequires: golang(github.com/docker/docker/pkg/parsers/kernel)
BuildRequires: golang(github.com/docker/docker/pkg/plugins)
BuildRequires: golang(github.com/docker/docker/pkg/pools)
BuildRequires: golang(github.com/docker/docker/pkg/progress)
BuildRequires: golang(github.com/docker/docker/pkg/promise)
BuildRequires: golang(github.com/docker/docker/pkg/random)
BuildRequires: golang(github.com/docker/docker/pkg/reexec)
BuildRequires: golang(github.com/docker/docker/pkg/stringid)
BuildRequires: golang(github.com/docker/docker/pkg/system)
BuildRequires: golang(github.com/docker/docker/pkg/tarsum)
BuildRequires: golang(github.com/docker/docker/pkg/term)
BuildRequires: golang(github.com/docker/docker/pkg/term/windows)
BuildRequires: golang(github.com/docker/docker/pkg/useragent)
BuildRequires: golang(github.com/docker/docker/pkg/version)
BuildRequires: golang(github.com/docker/docker/reference)
BuildRequires: golang(github.com/docker/docker/registry)
BuildRequires: golang(github.com/docker/engine-api/types)
BuildRequires: golang(github.com/docker/engine-api/types/blkiodev)
BuildRequires: golang(github.com/docker/engine-api/types/container)
BuildRequires: golang(github.com/docker/engine-api/types/filters)
BuildRequires: golang(github.com/docker/engine-api/types/image)
BuildRequires: golang(github.com/docker/engine-api/types/network)
BuildRequires: golang(github.com/docker/engine-api/types/registry)
BuildRequires: golang(github.com/docker/engine-api/types/strslice)
BuildRequires: golang(github.com/docker/go-connections/nat)
BuildRequires: golang(github.com/docker/go-connections/tlsconfig)
BuildRequires: golang(github.com/docker/go-units)
BuildRequires: golang(github.com/docker/libtrust)
BuildRequires: golang(github.com/gorilla/context)
BuildRequires: golang(github.com/gorilla/mux)
BuildRequires: golang(github.com/opencontainers/runc/libcontainer/user)
BuildRequires: golang(github.com/vbatts/tar-split/archive/tar)
BuildRequires: golang(github.com/vbatts/tar-split/tar/asm)
BuildRequires: golang(github.com/vbatts/tar-split/tar/storage)
BuildRequires: golang(golang.org/x/net/context)
%endif

%description devel
%{summary}

This package contains library source intended for
building other packages which use import path with
%{import_path} prefix.
%endif

%if 0%{?with_unit_test} && 0%{?with_devel}
%package unit-test-devel
Summary:         Unit tests for %{name} package
%if 0%{?with_check}
#Here comes all BuildRequires: PACKAGE the unit tests
#in %%check section need for running
%endif

# test subpackage tests code from devel subpackage
Requires:        %{name}-devel = %{version}-%{release}

%description unit-test-devel
%{summary}

This package contains unit tests for project
providing packages with %{import_path} prefix.
%endif

%package -n containers-common
Summary: Configuration files for working with image signatures
Obsoletes: atomic <= 1.13.1-2
Obsoletes: docker-rhsubscription <= 2:1.13.1-31
Obsoletes: skopeo-containers

%description -n containers-common
This package installs a default signature store configuration and a default
policy under `/etc/containers/`.

%prep
%autosetup -Sgit -n %{name}-%{commit0}

%build
mkdir -p src/github.com/projectatomic
ln -s ../../../ src/%{import_path}

mkdir -p vendor/src
for v in vendor/*; do
    if test ${v} = vendor/src; then continue; fi
    if test -d ${v}; then
	mv ${v} vendor/src/
    fi
done

%if ! 0%{?with_bundled}
rm -rf vendor/
export GOPATH=$(pwd):%{gopath}
%else
export GOPATH=$(pwd):$(pwd)/vendor:%{gopath}
%endif

make binary-local docs

%install
make DESTDIR=%{buildroot} install
mkdir -p %{buildroot}%{_sysconfdir}
install -m0644 %{SOURCE1} %{buildroot}%{_sysconfdir}/containers/storage.conf
mkdir -p %{buildroot}%{_mandir}/man5
go-md2man -in %{SOURCE2} -out %{buildroot}%{_mandir}/man5/containers-storage.conf.5
mkdir -p %{buildroot}%{_datadir}/containers
install -m0644 %{SOURCE3} %{buildroot}%{_datadir}/containers/mounts.conf

# install secrets patch directory
install -d -p -m 750 %{buildroot}/%{_datadir}/rhel/secrets
# rhbz#1110876 - update symlinks for subscription management
ln -s %{_sysconfdir}/pki/entitlement %{buildroot}%{_datadir}/rhel/secrets/etc-pki-entitlement
ln -s %{_sysconfdir}/rhsm %{buildroot}%{_datadir}/rhel/secrets/rhsm
ln -s %{_sysconfdir}/yum.repos.d/redhat.repo %{buildroot}%{_datadir}/rhel/secrets/rhel7.repo

# source codes for building projects
%if 0%{?with_devel}
install -d -p %{buildroot}/%{gopath}/src/%{import_path}/
echo "%%dir %%{gopath}/src/%%{import_path}/." >> devel.file-list
# find all *.go but no *_test.go files and generate devel.file-list
for file in $(find . -iname "*.go" \! -iname "*_test.go" | grep -v "./vendor") ; do
    echo "%%dir %%{gopath}/src/%%{import_path}/$(dirname $file)" >> devel.file-list
    install -d -p %{buildroot}/%{gopath}/src/%{import_path}/$(dirname $file)
    cp -pav $file %{buildroot}/%{gopath}/src/%{import_path}/$file
    echo "%%{gopath}/src/%%{import_path}/$file" >> devel.file-list
done
%endif

# testing files for this project
%if 0%{?with_unit_test} && 0%{?with_devel}
install -d -p %{buildroot}/%{gopath}/src/%{import_path}/
# find all *_test.go files and generate unit-test.file-list
for file in $(find . -iname "*_test.go" | grep -v "./vendor"); do
    echo "%%dir %%{gopath}/src/%%{import_path}/$(dirname $file)" >> devel.file-list
    install -d -p %{buildroot}/%{gopath}/src/%{import_path}/$(dirname $file)
    cp -pav $file %{buildroot}/%{gopath}/src/%{import_path}/$file
    echo "%%{gopath}/src/%%{import_path}/$file" >> unit-test-devel.file-list
done
%endif

%if 0%{?with_devel}
sort -u -o devel.file-list devel.file-list
%endif

%check
%if 0%{?with_check} && 0%{?with_unit_test} && 0%{?with_devel}
%if ! 0%{?with_bundled}
export GOPATH=%{buildroot}/%{gopath}:%{gopath}
%else
export GOPATH=%{buildroot}/%{gopath}:$(pwd)/vendor:%{gopath}
%endif

%gotest %{import_path}/integration
%endif

#define license tag if not already defined
%{!?_licensedir:%global license %doc}

%if 0%{?with_devel}
%files devel -f devel.file-list
%license LICENSE
%doc README.md
%dir %{gopath}/src/%{provider}.%{provider_tld}/%{project}
%endif

%if 0%{?with_unit_test} && 0%{?with_devel}
%files unit-test-devel -f unit-test-devel.file-list
%license LICENSE
%doc README.md
%endif

%files -n containers-common
%dir %{_sysconfdir}/containers
%dir %{_sysconfdir}/containers/registries.d
%config(noreplace) %{_sysconfdir}/containers/policy.json
%config(noreplace) %{_sysconfdir}/containers/registries.d/default.yaml
%config(noreplace) %{_sysconfdir}/containers/storage.conf 
%dir %{_sharedstatedir}/atomic/sigstore
%{_mandir}/man5/containers-storage.conf.5*
%dir %{_datadir}/containers
%{_datadir}/containers/mounts.conf
%dir %{_datadir}/rhel/secrets
%{_datadir}/rhel/secrets/etc-pki-entitlement
%{_datadir}/rhel/secrets/rhel7.repo
%{_datadir}/rhel/secrets/rhsm

%files
%license LICENSE
%doc README.md
%{_bindir}/%{name}
%{_mandir}/man1/%{name}.1*
%dir %{_datadir}/bash-completion
%dir %{_datadir}/bash-completion/completions
%{_datadir}/bash-completion/completions/%{name}

%changelog
* Tue Nov 21 2017 dwalsh <dwalsh@redhat.com> - 0.1.27-1.git
- Fix Conflicts to Obsoletes
- Add better docs to man pages.
- Use credentials from authfile for skopeo commands
- Support storage="" in /etc/containers/storage.conf
- Add global --override-arch and --override-os options

* Wed Nov 15 2017 dwalsh <dwalsh@redhat.com> - 0.1.25-2.git2e8377a7
-  Add manifest type conversion to skopeo copy
-  User can select from 3 manifest types: oci, v2s1, or v2s2
-   e.g skopeo copy --format v2s1 --compress-blobs docker-archive:alp.tar dir:my-directory

* Wed Nov 8 2017 dwalsh <dwalsh@redhat.com> - 0.1.25-2.git7fd6f66b
- Force storage.conf to default to overlay

* Wed Nov 8 2017 dwalsh <dwalsh@redhat.com> - 0.1.25-1.git7fd6f66b
-   Fix CVE in tar-split
-   copy: add shared blob directory support for OCI sources/destinations
-   Aligning Docker version between containers/image and skopeo
-   Update image-tools, and remove the duplicate Sirupsen/logrus vendor
-   makefile: use -buildmode=pie
  
* Tue Nov 7 2017 dwalsh <dwalsh@redhat.com> - 0.1.24-8.git28d4e08a
- Add /usr/share/containers/mounts.conf

* Sun Oct 22 2017 dwalsh <dwalsh@redhat.com> - 0.1.24-7.git28d4e08a
- Bug fixes
- Update to release

* Tue Oct 17 2017 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.1.24-6.dev.git28d4e08
- skopeo-containers conflicts with docker-rhsubscription <= 2:1.13.1-31

* Tue Oct 17 2017 dwalsh <dwalsh@redhat.com> - 0.1.24-5.dev.git28d4e08
- Add rhel subscription secrets data to skopeo-containers

* Thu Oct 12 2017 dwalsh <dwalsh@redhat.com> - 0.1.24-4.dev.git28d4e08
- Update container/storage.conf and containers-storage.conf man page
- Default override to true so it is consistent with RHEL.

* Tue Oct 10 2017 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.1.24-3.dev.git28d4e08
- built commit 28d4e08

* Mon Sep 18 2017 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.1.24-2.dev.git875dd2e
- built commit 875dd2e
- Resolves: gh#416

* Tue Sep 12 2017 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.1.24-1.dev.gita41cd0
- bump to 0.1.24-dev
- correct a prior bogus date
- fix macro in comment warning

* Mon Aug 21 2017 dwalsh <dwalsh@redhat.com> - 0.1.23-6.dev.git1bbd87
- Change name of storage.conf.5 man page to containers-storage.conf.5, since
it conflicts with inn package
- Also remove default to "overalay" in the configuration, since we should
- allow containers storage to pick the best default for the platform.

* Thu Aug 03 2017 Fedora Release Engineering <releng@fedoraproject.org> - 0.1.23-5.git1bbd87f
- Rebuilt for https://fedoraproject.org/wiki/Fedora_27_Binutils_Mass_Rebuild

* Sun Jul 30 2017 Florian Weimer <fweimer@redhat.com> - 0.1.23-4.git1bbd87f
- Rebuild with binutils fix for ppc64le (#1475636)

* Thu Jul 27 2017 Fedora Release Engineering <releng@fedoraproject.org> - 0.1.23-3.git1bbd87f
- Rebuilt for https://fedoraproject.org/wiki/Fedora_27_Mass_Rebuild

* Tue Jul 25 2017 dwalsh <dwalsh@redhat.com> - 0.1.23-2.dev.git1bbd87
- Fix storage.conf man page to be storage.conf.5.gz so that it works.

* Fri Jul 21 2017 dwalsh <dwalsh@redhat.com> - 0.1.23-1.dev.git1bbd87
- Support for OCI V1.0 Images
- Update to image-spec v1.0.0 and revendor
- Fixes for authentication

* Sat Jul 01 2017 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.1.22-2.dev.git5d24b67
- Epoch: 1 for CentOS as CentOS Extras' build already has epoch set to 1

* Wed Jun 21 2017 dwalsh <dwalsh@redhat.com> - 0.1.22-1.dev.git5d24b67
-  Give more useful help when explaining usage
-  Also specify container-storage as a valid transport
-  Remove docker reference wherever possible
-  vendor in ostree fixes

* Thu Jun 15 2017 dwalsh <dwalsh@redhat.com> - 0.1.21-1.dev.git0b73154
- Add support for storage.conf and storage-config.5.md from github container storage package
- Bump to the latest version of skopeo
- vendor.conf: add ostree-go
-       it is used by containers/image for pulling images to the OSTree storage.
- fail early when image os does not match host os
- Improve documentation on what to do with containers/image failures in test-skopeo
-   We now have the docker-archive: transport
-   Integration tests with built registries also exist
- Support /etc/docker/certs.d
- update image-spec to v1.0.0-rc6

* Tue May 23 2017 bbaude <bbaude@redhat.com> - 0.1.20-1.dev.git0224d8c
- BZ #1380078 - New release

* Tue Apr 25 2017 bbaude <bbaude@redhat.com> - 0.1.19-2.dev.git0224d8c
- No golang support for ppc64.  Adding exclude arch. BZ #1445490

* Tue Feb 28 2017 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.1.19-1.dev.git0224d8c
- bump to v0.1.19-dev
- built commit 0224d8c

* Sat Feb 11 2017 Fedora Release Engineering <releng@fedoraproject.org> - 0.1.17-3.dev.git2b3af4a
- Rebuilt for https://fedoraproject.org/wiki/Fedora_26_Mass_Rebuild

* Sat Dec 10 2016 Igor Gnatenko <i.gnatenko.brain@gmail.com> - 0.1.17-2.dev.git2b3af4a
- Rebuild for gpgme 1.18

* Tue Dec 06 2016 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.1.17-1.dev.git2b3af4a
- bump to 0.1.17-dev

* Fri Nov 04 2016 Antonio Murdaca <runcom@fedoraproject.org> - 0.1.14-6.git550a480
- Fix BZ#1391932

* Tue Oct 18 2016 Antonio Murdaca <runcom@fedoraproject.org> - 0.1.14-5.git550a480
- Conflicts with atomic in skopeo-containers

* Wed Oct 12 2016 Antonio Murdaca <runcom@fedoraproject.org> - 0.1.14-4.git550a480
- built skopeo-containers

* Wed Sep 21 2016 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.1.14-3.gitd830391
- built mtrmac/integrate-all-the-things commit d830391

* Thu Sep 08 2016 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.1.14-2.git362bfc5
- built commit 362bfc5

* Thu Aug 11 2016 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.1.14-1.gitffe92ed
- build origin/master commit ffe92ed

* Thu Jul 21 2016 Fedora Release Engineering <rel-eng@lists.fedoraproject.org> - 0.1.13-6
- https://fedoraproject.org/wiki/Changes/golang1.7

* Tue Jun 21 2016 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.1.13-5
- include go-srpm-macros and compiler(go-compiler) in fedora conditionals
- define %%gobuild if not already
- add patch to build with older version of golang

* Thu Jun 02 2016 Antonio Murdaca <runcom@fedoraproject.org> - 0.1.13-4
- update to v0.1.12

* Tue May 31 2016 Antonio Murdaca <runcom@fedoraproject.org> - 0.1.12-3
- fix go build source path

* Fri May 27 2016 Antonio Murdaca <runcom@fedoraproject.org> - 0.1.12-2
- update to v0.1.12

* Tue Mar 08 2016 Antonio Murdaca <runcom@fedoraproject.org> - 0.1.11-1
- update to v0.1.11

* Tue Mar 08 2016 Antonio Murdaca <runcom@fedoraproject.org> - 0.1.10-1
- update to v0.1.10
- change runcom -> projectatomic

* Mon Feb 29 2016 Antonio Murdaca <runcom@fedoraproject.org> - 0.1.9-1
- update to v0.1.9

* Mon Feb 29 2016 Antonio Murdaca <runcom@fedoraproject.org> - 0.1.8-1
- update to v0.1.8

* Mon Feb 22 2016 Fedora Release Engineering <rel-eng@lists.fedoraproject.org> - 0.1.4-2
- https://fedoraproject.org/wiki/Changes/golang1.6

* Fri Jan 29 2016 Antonio Murdaca <runcom@redhat.com> - 0.1.4
- First package for Fedora
