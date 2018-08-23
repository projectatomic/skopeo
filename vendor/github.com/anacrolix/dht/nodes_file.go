package dht

import (
	"io/ioutil"
	"os"

	"github.com/anacrolix/dht/krpc"
)

func WriteNodesToFile(ns []krpc.NodeInfo, fileName string) (err error) {
	b, err := krpc.CompactIPv6NodeInfo(ns).MarshalBinary()
	if err != nil {
		return
	}
	f, err := os.OpenFile(fileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0640)
	if err != nil {
		return
	}
	defer func() {
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
	}()
	_, err = f.Write(b)
	return
}

func ReadNodesFromFile(fileName string) (ns []krpc.NodeInfo, err error) {
	f, err := os.Open(fileName)
	if err != nil {
		return
	}
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return
	}
	var cnis krpc.CompactIPv6NodeInfo
	err = cnis.UnmarshalBinary(b)
	ns = cnis
	return
}
