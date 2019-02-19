#!/bin/bash
if pkg-config ostree-1 2> /dev/null ; then
	echo ostree ostree_repos
fi
