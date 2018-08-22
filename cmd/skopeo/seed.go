package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/containers/image/torrent"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func seedHandler(c *cli.Context) error {
	trackers := c.StringSlice("torrent-trackers")

	ctx := &types.SystemContext{
		DockerTorrentTrackers: trackers,
	}

	if len(c.Args()) < 2 || len(c.Args())%2 != 0 {
		cli.ShowCommandHelp(c, "seed")
		return errors.New("Invalid number of arguments")
	}

	debug := logrus.GetLevel() == logrus.DebugLevel

	client, err := torrent.MakeClient(ctx, debug, true, 0)
	if err != nil {
		return err
	}
	defer client.Close()

	for off := 0; off < len(c.Args()); off = off + 2 {
		ref, err := alltransports.ParseImageName(c.Args()[off])
		if err != nil {
			return fmt.Errorf("Invalid source image %s: %v", c.Args()[off], err)
		}

		refSrc, err := alltransports.ParseImageName(c.Args()[off+1])
		if err != nil {
			return fmt.Errorf("Invalid storage image %s: %v", c.Args()[off+1], err)
		}

		if err := client.Seed(context.Background(), ctx, ref, refSrc); err != nil {
			return err
		}
	}
	if debug {
		go func() {
			for {
				time.Sleep(time.Second * 10)
				client.WriteStatus(os.Stderr)
			}
		}()
	}

	s := make(chan os.Signal)
	signal.Notify(s, os.Interrupt, syscall.SIGTERM)
	<-s
	logrus.Info("got signal, shut down")
	client.Close()
	return nil
}

func seedCmd() cli.Command {
	return cli.Command{
		Name:        "seed",
		Usage:       "Seed images on BitTorrent",
		Description: "Seed multiple images on BitTorrent",
		ArgsUsage:   "REMOTE_IMAGE REF_IMAGE [REMOTE_IMAGE REF_IMAGE...]",
		Action:      seedHandler,
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  "torrent-trackers",
				Usage: "additional trackers for BitTorrent",
			},
		},
	}
}
