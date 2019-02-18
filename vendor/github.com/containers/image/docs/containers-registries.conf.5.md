% CONTAINERS-REGISTRIES.CONF(5) System-wide registry configuration file
% Brent Baude
% Aug 2017

# NAME
containers-registries.conf - Syntax of System Registry Configuration File

# DESCRIPTION
The CONTAINERS-REGISTRIES configuration file is a system-wide configuration
file for container image registries. The file format is TOML. The valid
categories are: 'registries.search', 'registries.insecure', and
'registries.block'.

By default, the configuration file is located at `/etc/containers/registries.conf`.

# FORMAT
The TOML_format is used to build a simple list format for registries under three
categories: `registries.search`, `registries.insecure`, and `registries.block`.
You can list multiple registries using a comma separated list.

Search registries are used when the caller of a container runtime does not fully specify the
container image that they want to execute.  These registries are prepended onto the front
of the specified container image until the named image is found at a registry.

Insecure Registries.  By default container runtimes use TLS when retrieving images
from a registry.  If the registry is not setup with TLS, then the container runtime
will fail to pull images from the registry. If you add the registry to the list of
insecure registries then the container runtime will attempt use standard web protocols to
pull the image.  It also allows you to pull from a registry with self-signed certificates.
Note insecure registries can be used for any registry, not just the registries listed
under search.

Block Registries.  The registries in this category are are not pulled from when
retrieving images.

# EXAMPLE
The following example configuration defines two searchable registries, one
insecure registry, and two blocked registries.

```
[registries.search]
registries = ['registry1.com', 'registry2.com']

[registries.insecure]
registries = ['registry3.com']

[registries.block]
registries = ['registry.untrusted.com', 'registry.unsafe.com']
```

# HISTORY
Aug 2018, Renamed to containers-registries.conf(5) by Valentin Rothberg <vrothberg@suse.com>

Jun 2018, Updated by Tom Sweeney <tsweeney@redhat.com>

Aug 2017, Originally compiled by Brent Baude <bbaude@redhat.com>
