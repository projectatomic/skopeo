Bencode encoding/decoding sub package. Uses similar API design to Go's json package.

## Install

```sh
go get github.com/anacrolix/torrent
```

## Usage

```go
package demo

import (
	bencode "github.com/anacrolix/torrent/bencode"
)

type Message struct {
	Query    string `json:"q,omitempty" bencode:"q,omitempty"`
}

var v Message

func main(){
	// encode
	data, err := bencode.Marshal(v)
	if err != nil {
		log.Fatal(err)
	}
	
	//decode
	err := bencode.Unmarshal(data, &v)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(v)
}
```
