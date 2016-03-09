package main

import (
	"encoding/json"
	"fmt"

	"github.com/projectatomic/skopeo/docker"
	"github.com/projectatomic/skopeo/types"
)

func inspect(img string, kind Kind) ([]byte, error) {
	var (
		imgInspect *types.ImageInspect
		err        error
	)
	switch kind {
	case KindDocker:
		imgInspect, err = docker.GetData(img)
		if err != nil {
			return nil, err
		}
	case KindAppc:
		return nil, fmt.Errorf("not implemented yet")
	}
	out, err := json.Marshal(imgInspect)
	if err != nil {
		return nil, err
	}
	return out, nil
}
