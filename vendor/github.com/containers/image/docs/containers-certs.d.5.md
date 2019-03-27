% containers-certs.d(5)

# NAME
containers-certs.d - Directory for storing custom container-registry TLS configurations

# DESCRIPTION
A custom TLS configuration for a container registry can be configured by creating a directory under `/etc/containers/certs.d`.
The name of the directory must correspond to the `host:port` of the registry (e.g., `my-registry.com:5000`).

## Directory Structure
A certs directory can contain one or more files with the following extensions:

* `*.crt`  files with this extensions will be interpreted as CA certificates
* `*.cert` files with this extensions will be interpreted as client certificates
* `*.key`  files with this extensions will be interpreted as client keys

Note that the client certificate-key pair will be selected by the file name (e.g., `client.{cert,key}`).
An examplary setup for a registry running at `my-registry.com:5000` may look as follows:
```
/etc/containers/certs.d/    <- Certificate directory
└── my-registry.com:5000    <- Hostname:port
   ├── client.cert          <- Client certificate
   ├── client.key           <- Client key
   └── ca.crt               <- Certificate authority that signed the registry certificate
```

# HISTORY
Feb 2019, Originally compiled by Valentin Rothberg <rothberg@redhat.com>
