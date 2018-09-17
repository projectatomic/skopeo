package main

import (
	"io"
	"os"

	"github.com/proglottis/gpgme"
)

func main() {
	plain, err := gpgme.Decrypt(os.Stdin)
	if err != nil {
		panic(err)
	}
	defer plain.Close()
	if _, err := io.Copy(os.Stdout, plain); err != nil {
		panic(err)
	}
}
