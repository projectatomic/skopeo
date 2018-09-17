package main

import (
	"flag"
	"os"

	"github.com/proglottis/gpgme"
)

func main() {
	flag.Parse()
	filter := flag.Arg(0)
	if filter == "" {
		panic("must specify recipient filter")
	}
	recipients, err := gpgme.FindKeys(filter, false)
	if err != nil {
		panic(err)
	}
	if len(recipients) < 1 {
		panic("no keys match")
	}
	plain, err := gpgme.NewDataReader(os.Stdin)
	if err != nil {
		panic(err)
	}
	cipher, err := gpgme.NewDataWriter(os.Stdout)
	if err != nil {
		panic(err)
	}
	ctx, err := gpgme.New()
	if err != nil {
		panic(err)
	}
	ctx.SetArmor(true)
	if err := ctx.Encrypt(recipients, 0, plain, cipher); err != nil {
		panic(err)
	}
}
