// +build selinux,linux

package selinux

import (
	"bufio"
	"bytes"
	"os"
	"testing"
)

func TestSetFileLabel(t *testing.T) {
	if !GetEnabled() {
		t.Skip("SELinux not enabled, skipping.")
	}

	tmp := "selinux_test"
	con := "system_u:object_r:bin_t:s0"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE, 0)
	if err != nil {
		t.Fatalf("unable to open %s: %s", tmp, err)
	}
	out.Close()
	defer os.Remove(tmp)

	if err := SetFileLabel(tmp, con); err != nil {
		t.Fatalf("SetFileLabel failed: %s", err)
	}
	filelabel, err := FileLabel(tmp)
	if err != nil {
		t.Fatalf("FileLabel failed: %s", err)
	}
	if con != filelabel {
		t.Fatalf("FileLabel failed, returned %s expected %s", filelabel, con)
	}
}

func TestSELinux(t *testing.T) {
	if !GetEnabled() {
		t.Skip("SELinux not enabled, skipping.")
	}

	var (
		err            error
		plabel, flabel string
	)

	plabel, flabel = ContainerLabels()
	t.Log(plabel)
	t.Log(flabel)
	ReleaseLabel(plabel)
	plabel, flabel = ContainerLabels()
	t.Log(plabel)
	t.Log(flabel)
	ReleaseLabel(plabel)
	t.Log("Enforcing Mode", EnforceMode())
	mode := DefaultEnforceMode()
	t.Log("Default Enforce Mode ", mode)

	defer SetEnforceMode(mode)
	if err := SetEnforceMode(Enforcing); err != nil {
		t.Fatalf("enforcing selinux failed: %v", err)
	}
	if err := SetEnforceMode(Permissive); err != nil {
		t.Fatalf("setting selinux mode to permissive failed: %v", err)
	}
	SetEnforceMode(mode)

	pid := os.Getpid()
	t.Logf("PID:%d MCS:%s\n", pid, intToMcs(pid, 1023))
	err = SetFSCreateLabel("unconfined_u:unconfined_r:unconfined_t:s0")
	if err == nil {
		t.Log(FSCreateLabel())
	} else {
		t.Log("SetFSCreateLabel failed", err)
		t.Fatal(err)
	}
	err = SetFSCreateLabel("")
	if err == nil {
		t.Log(FSCreateLabel())
	} else {
		t.Log("SetFSCreateLabel failed", err)
		t.Fatal(err)
	}
	t.Log(PidLabel(1))
}

func TestCanonicalizeContext(t *testing.T) {
	if !GetEnabled() {
		t.Skip("SELinux not enabled, skipping.")
	}

	con := "system_u:object_r:bin_t:s0:c1,c2,c3"
	checkcon := "system_u:object_r:bin_t:s0:c1.c3"
	newcon, err := CanonicalizeContext(con)
	if err != nil {
		t.Fatal(err)
	}
	if newcon != checkcon {
		t.Fatalf("CanonicalizeContext(%s) returned %s expected %s", con, newcon, checkcon)
	}
	con = "system_u:object_r:bin_t:s0:c5,c2"
	checkcon = "system_u:object_r:bin_t:s0:c2,c5"
	newcon, err = CanonicalizeContext(con)
	if err != nil {
		t.Fatal(err)
	}
	if newcon != checkcon {
		t.Fatalf("CanonicalizeContext(%s) returned %s expected %s", con, newcon, checkcon)
	}
}

func TestFindSELinuxfsInMountinfo(t *testing.T) {
	const mountinfo = `18 62 0:17 / /sys rw,nosuid,nodev,noexec,relatime shared:6 - sysfs sysfs rw,seclabel
19 62 0:3 / /proc rw,nosuid,nodev,noexec,relatime shared:5 - proc proc rw
20 62 0:5 / /dev rw,nosuid shared:2 - devtmpfs devtmpfs rw,seclabel,size=3995472k,nr_inodes=998868,mode=755
21 18 0:16 / /sys/kernel/security rw,nosuid,nodev,noexec,relatime shared:7 - securityfs securityfs rw
22 20 0:18 / /dev/shm rw,nosuid,nodev shared:3 - tmpfs tmpfs rw,seclabel
23 20 0:11 / /dev/pts rw,nosuid,noexec,relatime shared:4 - devpts devpts rw,seclabel,gid=5,mode=620,ptmxmode=000
24 62 0:19 / /run rw,nosuid,nodev shared:23 - tmpfs tmpfs rw,seclabel,mode=755
25 18 0:20 / /sys/fs/cgroup ro,nosuid,nodev,noexec shared:8 - tmpfs tmpfs ro,seclabel,mode=755
26 25 0:21 / /sys/fs/cgroup/systemd rw,nosuid,nodev,noexec,relatime shared:9 - cgroup cgroup rw,xattr,release_agent=/usr/lib/systemd/systemd-cgroups-agent,name=systemd
27 18 0:22 / /sys/fs/pstore rw,nosuid,nodev,noexec,relatime shared:20 - pstore pstore rw
28 25 0:23 / /sys/fs/cgroup/perf_event rw,nosuid,nodev,noexec,relatime shared:10 - cgroup cgroup rw,perf_event
29 25 0:24 / /sys/fs/cgroup/devices rw,nosuid,nodev,noexec,relatime shared:11 - cgroup cgroup rw,devices
30 25 0:25 / /sys/fs/cgroup/cpu,cpuacct rw,nosuid,nodev,noexec,relatime shared:12 - cgroup cgroup rw,cpuacct,cpu
31 25 0:26 / /sys/fs/cgroup/freezer rw,nosuid,nodev,noexec,relatime shared:13 - cgroup cgroup rw,freezer
32 25 0:27 / /sys/fs/cgroup/net_cls,net_prio rw,nosuid,nodev,noexec,relatime shared:14 - cgroup cgroup rw,net_prio,net_cls
33 25 0:28 / /sys/fs/cgroup/cpuset rw,nosuid,nodev,noexec,relatime shared:15 - cgroup cgroup rw,cpuset
34 25 0:29 / /sys/fs/cgroup/memory rw,nosuid,nodev,noexec,relatime shared:16 - cgroup cgroup rw,memory
35 25 0:30 / /sys/fs/cgroup/pids rw,nosuid,nodev,noexec,relatime shared:17 - cgroup cgroup rw,pids
36 25 0:31 / /sys/fs/cgroup/hugetlb rw,nosuid,nodev,noexec,relatime shared:18 - cgroup cgroup rw,hugetlb
37 25 0:32 / /sys/fs/cgroup/blkio rw,nosuid,nodev,noexec,relatime shared:19 - cgroup cgroup rw,blkio
59 18 0:33 / /sys/kernel/config rw,relatime shared:21 - configfs configfs rw
62 1 253:1 / / rw,relatime shared:1 - ext4 /dev/vda1 rw,seclabel,data=ordered
38 18 0:15 / /sys/fs/selinux rw,relatime shared:22 - selinuxfs selinuxfs rw
39 19 0:35 / /proc/sys/fs/binfmt_misc rw,relatime shared:24 - autofs systemd-1 rw,fd=29,pgrp=1,timeout=0,minproto=5,maxproto=5,direct,pipe_ino=11601
40 20 0:36 / /dev/hugepages rw,relatime shared:25 - hugetlbfs hugetlbfs rw,seclabel
41 20 0:14 / /dev/mqueue rw,relatime shared:26 - mqueue mqueue rw,seclabel
42 18 0:6 / /sys/kernel/debug rw,relatime shared:27 - debugfs debugfs rw
112 62 253:1 /var/lib/docker/plugins /var/lib/docker/plugins rw,relatime - ext4 /dev/vda1 rw,seclabel,data=ordered
115 62 253:1 /var/lib/docker/overlay2 /var/lib/docker/overlay2 rw,relatime - ext4 /dev/vda1 rw,seclabel,data=ordered
118 62 7:0 / /root/mnt rw,relatime shared:66 - ext4 /dev/loop0 rw,seclabel,data=ordered
121 115 0:38 / /var/lib/docker/overlay2/8cdbabf81bc89b14ea54eaf418c1922068f06917fff57e184aa26541ff291073/merged rw,relatime - overlay overlay rw,seclabel,lowerdir=/var/lib/docker/overlay2/l/CPD4XI7UD4GGTGSJVPQSHWZKTK:/var/lib/docker/overlay2/l/NQKORR3IS7KNQDER35AZECLH4Z,upperdir=/var/lib/docker/overlay2/8cdbabf81bc89b14ea54eaf418c1922068f06917fff57e184aa26541ff291073/diff,workdir=/var/lib/docker/overlay2/8cdbabf81bc89b14ea54eaf418c1922068f06917fff57e184aa26541ff291073/work
125 62 0:39 / /var/lib/docker/containers/5e3fce422957c291a5b502c2cf33d512fc1fcac424e4113136c808360e5b7215/shm rw,nosuid,nodev,noexec,relatime shared:68 - tmpfs shm rw,seclabel,size=65536k
186 24 0:3 / /run/docker/netns/0a08e7496c6d rw,nosuid,nodev,noexec,relatime shared:5 - proc proc rw
130 62 0:15 / /root/chroot/selinux rw,relatime shared:22 - selinuxfs selinuxfs rw
109 24 0:37 / /run/user/0 rw,nosuid,nodev,relatime shared:62 - tmpfs tmpfs rw,seclabel,size=801032k,mode=700
`
	s := bufio.NewScanner(bytes.NewBuffer([]byte(mountinfo)))
	for _, expected := range []string{"/sys/fs/selinux", "/root/chroot/selinux", ""} {
		mnt := findSELinuxfsMount(s)
		t.Logf("found %q", mnt)
		if mnt != expected {
			t.Fatalf("expected %q, got %q", expected, mnt)
		}
	}
}
