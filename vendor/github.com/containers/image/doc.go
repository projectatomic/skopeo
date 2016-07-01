// Package image provides libraries and commands to interact with containers images.
//
//    package main
//
//    import (
//    	"fmt"
//
//    	"github.com/containers/image/docker"
//    )
//
//    func main() {
//    	img, err := docker.NewDockerImage("fedora", "", false)
//    	if err != nil {
//    		panic(err)
//    	}
//    	b, err := img.Manifest()
//    	if err != nil {
//    		panic(err)
//    	}
//    	fmt.Printf("%s", string(b))
//    }
//
// TODO(runcom)
package image
