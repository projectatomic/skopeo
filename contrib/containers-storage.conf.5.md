% containers-storage.conf(5) Container Storage Configuration File
% Dan Walsh
% May 2017

# NAME
storage.conf - Syntax of Container Storage configuration file

## DESCRIPTION
The STORAGE configuration file specifies all of the available container storage options
for tools using shared container storage, but in a TOML format that can be more easily modified
and versioned.

## FORMAT
The [TOML format][toml] is used as the encoding of the configuration file.
Every option and subtable listed here is nested under a global "storage" table.
No bare options are used. The format of TOML can be simplified to:

    [table]
    option = value

    [table.subtable1]
    option = value

    [table.subtable2]
    option = value

## STORAGE TABLE

The `storage` table supports the following options:

**driver**=""
  container storage driver (default: "overlay")
  Default Copy On Write (COW) container storage driver
  Valid drivers are "overlay", "vfs", "devmapper", "aufs", "btrfs", and "zfs"
  Some drivers (for example, "zfs", "btrfs", and "aufs") may not work if your kernel lacks support for the filesystem

**graphroot**=""
  container storage graph dir (default: "/var/lib/containers/storage")
  Default directory to store all writable content created by container storage programs

**runroot**=""
  container storage run dir (default: "/var/run/containers/storage")
  Default directory to store all temporary writable content created by container storage programs

### STORAGE OPTIONS TABLE

The `storage.options` table supports the following options:

**additionalimagestores**=[]
  Paths to additional container image stores. Usually these are read/only and stored on remote network shares.

**ignore_chown_errors** = "true|False"
  ignore_chown_errors can be set to allow a non privileged user running with a  single UID within a user namespace to run containers. The user can pull and use any image even those with multiple uids.  Note multiple UIDs will be squasheddown to the default uid in the container.  These images will have no separation between the users in the container. Only supported for the overlay and vfs drivers.

**mount_program**=""
  Specifies the path to a custom program to use instead of using kernel defaults for mounting the file system.

      mount_program = "/usr/bin/fuse-overlayfs"

**mountopt**=""

  Comma separated list of default options to be used to mount container images.  Suggested value "nodev".

**ostree_repo** = ""
   If specified, use OSTree to deduplicate files with the overlay or vfs backends.

**size**=""
  Maximum size of a container image.   This flag can be used to set quota on the size of container images. (default: 10GB)

**skip_mount_home** = "false"
   Set to skip a PRIVATE bind mount on the storage home directory.
Only supported by certain container storage drivers (overlay).

**remap-uids=**""
**remap-gids=**""

  Remap-UIDs/GIDs is the mapping from UIDs/GIDs as they should appear inside of
a container, to the UIDs/GIDs outside of the container, and the length of the
range of UIDs/GIDs.  Additional mapped sets can be listed and will be heeded by
libraries, but there are limits to the number of mappings which the kernel will
allow when you later attempt to run a container.

     Example
     remap-uids = 0:1668442479:65536
     remap-gids = 0:1668442479:65536

     These mappings tell the container engines to map UID 0 inside of the
     container to UID 1668442479 outside.  UID 1 will be mapped to 1668442480.
     UID 2 will be mapped to 1668442481, etc, for the next 65533 UIDs in
     Succession.

**remap-user**=""
**remap-group**=""

  Remap-User/Group is a user name which can be used to look up one or more UID/GID
ranges in the /etc/subuid or /etc/subgid file.  Mappings are set up starting
with an in-container ID of 0 and then a host-level ID taken from the lowest
range that matches the specified name, and using the length of that range.
Additional ranges are then assigned, using the ranges which specify the
lowest host-level IDs first, to the lowest not-yet-mapped in-container ID,
until all of the entries have been used for maps.

      remap-user = "storage"
      remap-group = "storage"

### STORAGE OPTIONS FOR THINPOOL TABLE

The `storage.options.thinpool` table supports the following options:

**autoextend_percent**=""

Tells the thinpool driver the amount by which the thinpool needs to be grown. This is specified in terms of % of pool size. So a value of 20 means that when threshold is hit, pool will be grown by 20% of existing pool size. (default: 20%)

**autoextend_threshold**=""

Tells the driver the thinpool extension threshold in terms of percentage of pool size. For example, if threshold is 60, that means when pool is 60% full, threshold has been hit. (default: 80%)

**basesize**=""

Specifies the size to use when creating the base device, which limits the size of images and containers. (default: 10g)

**blocksize**=""

Specifies a custom blocksize to use for the thin pool. (default: 64k)

**directlvm_device**=""

Specifies a custom block storage device to use for the thin pool. Required for using graphdriver `devicemapper`.

**directlvm_device_force**=""

Tells driver to wipe device (directlvm_device) even if device already has a filesystem.  (default: false)

**fs**="xfs"

Specifies the filesystem type to use for the base device. (default: xfs)

**log_level**=""

Sets the log level of devicemapper.

    0: LogLevelSuppress 0 (default)
    2: LogLevelFatal
    3: LogLevelErr
    4: LogLevelWarn
    5: LogLevelNotice
    6: LogLevelInfo
    7: LogLevelDebug

**min_free_space**=""

Specifies the min free space percent in a thin pool required for new device creation to succeed. Valid values are from 0% - 99%. Value 0% disables. (default: 10%)

**mkfsarg**=""

Specifies extra mkfs arguments to be used when creating the base device.

**use_deferred_deletion**=""

Marks thinpool device for deferred deletion. If the thinpool is in use when the driver attempts to delete it, the driver will attempt to delete device every 30 seconds until successful, or when it restarts.  Deferred deletion permanently deletes the device and all data stored in the device will be lost. (default: true).

**use_deferred_removal**=""

Marks devicemapper block device for deferred removal.  If the device is in use when its driver attempts to remove it, the driver tells the kernel to remove the device as soon as possible.  Note this does not free up the disk space, use deferred deletion to fully remove the thinpool.  (default: true).

**xfs_nospace_max_retries**=""

Specifies the maximum number of retries XFS should attempt to complete IO when ENOSPC (no space) error is returned by underlying storage device. (default: 0, which means to try continuously.)

## SELINUX LABELING

When running on an SELinux system, if you move the containers storage graphroot directory, you must make sure the labeling is correct.

Tell SELinux about the new containers storage by setting up an equivalence record.
This tells SELinux to label content under the new path, as if it was stored
under `/var/lib/containers/storage`.

```
semanage fcontext -a -e /var/lib/containers NEWSTORAGEPATH
restorecon -R -v NEWSTORAGEPATH
```

The semanage command above tells SELinux to setup the default labeling of
`NEWSTORAGEPATH` to match `/var/lib/containers`.  The `restorecon` command
tells SELinux to apply the labels to the actual content.

Now all new content created in these directories will automatically be created
with the correct label.

## SEE ALSO
`semanage(8)`, `restorecon(8)`

## FILES

Distributions often provide a /usr/share/containers/storage.conf file to define default storage configuration. Administrators can override this file by creating `/etc/containers/storage.conf` to specify their own configuration. The storage.conf file for rootless users is stored in the $HOME/.config/containers/storage.conf file.

## HISTORY
May 2017, Originally compiled by Dan Walsh <dwalsh@redhat.com>
Format copied from crio.conf man page created by Aleksa Sarai <asarai@suse.de>
