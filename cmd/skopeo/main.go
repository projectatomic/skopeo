package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/containers/image/v5/signature"
	"github.com/containers/skopeo/version"
	"github.com/containers/storage/pkg/reexec"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// gitCommit will be the hash that the binary was built from
// and will be populated by the Makefile
var gitCommit = ""

type globalOptions struct {
	debug              bool          // Enable debug output
	tlsVerify          optionalBool  // Require HTTPS and verify certificates (for docker: and docker-daemon:)
	policyPath         string        // Path to a signature verification policy file
	insecurePolicy     bool          // Use an "allow everything" signature verification policy
	registriesDirPath  string        // Path to a "registries.d" registry configuration directory
	overrideArch       string        // Architecture to use for choosing images, instead of the runtime one
	overrideOS         string        // OS to use for choosing images, instead of the runtime one
	overrideVariant    string        // Architecture variant to use for choosing images, instead of the runtime one
	commandTimeout     time.Duration // Timeout for the command execution
	registriesConfPath string        // Path to the "registries.conf" file
}

// createApp returns a cli.App, and the underlying globalOptions object, to be run or tested.
func createApp() (*cli.App, *globalOptions) {
	opts := globalOptions{}

	app := cli.NewApp()
	app.EnableBashCompletion = true
	app.Name = "skopeo"
	if gitCommit != "" {
		app.Version = fmt.Sprintf("%s commit: %s", version.Version, gitCommit)
	} else {
		app.Version = version.Version
	}
	app.Usage = "Various operations with container images and container image registries"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "debug",
			Usage:       "enable debug output",
			Destination: &opts.debug,
		},
		cli.GenericFlag{
			Name:   "tls-verify",
			Usage:  "require HTTPS and verify certificates when talking to container registries (defaults to true)",
			Hidden: true,
			Value:  newOptionalBoolValue(&opts.tlsVerify),
		},
		cli.StringFlag{
			Name:        "policy",
			Usage:       "Path to a trust policy file",
			Destination: &opts.policyPath,
		},
		cli.BoolFlag{
			Name:        "insecure-policy",
			Usage:       "run the tool without any policy check",
			Destination: &opts.insecurePolicy,
		},
		cli.StringFlag{
			Name:        "registries.d",
			Usage:       "use registry configuration files in `DIR` (e.g. for container signature storage)",
			Destination: &opts.registriesDirPath,
		},
		cli.StringFlag{
			Name:        "override-arch",
			Usage:       "use `ARCH` instead of the architecture of the machine for choosing images",
			Destination: &opts.overrideArch,
		},
		cli.StringFlag{
			Name:        "override-os",
			Usage:       "use `OS` instead of the running OS for choosing images",
			Destination: &opts.overrideOS,
		},
		cli.StringFlag{
			Name:        "override-variant",
			Usage:       "use `VARIANT` instead of the running architecture variant for choosing images",
			Destination: &opts.overrideVariant,
		},
		cli.DurationFlag{
			Name:        "command-timeout",
			Usage:       "timeout for the command execution",
			Destination: &opts.commandTimeout,
		},
		cli.StringFlag{
			Name:        "registries-conf",
			Usage:       "path to the registries.conf file",
			Destination: &opts.registriesConfPath,
			Hidden:      true,
		},
	}
	app.Before = opts.before
	app.Commands = []cli.Command{
		copyCmd(&opts),
		inspectCmd(&opts),
		layersCmd(&opts),
		deleteCmd(&opts),
		manifestDigestCmd(),
		syncCmd(&opts),
		standaloneSignCmd(),
		standaloneVerifyCmd(),
		untrustedSignatureDumpCmd(),
		tagsCmd(&opts),
	}
	return app, &opts
}

// before is run by the cli package for any command, before running the command-specific handler.
func (opts *globalOptions) before(ctx *cli.Context) error {
	if opts.debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
	if opts.tlsVerify.present {
		logrus.Warn("'--tls-verify' is deprecated, please set this on the specific subcommand")
	}
	return nil
}

func main() {
	if reexec.Init() {
		return
	}
	app, _ := createApp()
	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

// getPolicyContext returns a *signature.PolicyContext based on opts.
func (opts *globalOptions) getPolicyContext() (*signature.PolicyContext, error) {
	var policy *signature.Policy // This could be cached across calls in opts.
	var err error
	if opts.insecurePolicy {
		policy = &signature.Policy{Default: []signature.PolicyRequirement{signature.NewPRInsecureAcceptAnything()}}
	} else if opts.policyPath == "" {
		policy, err = signature.DefaultPolicy(nil)
	} else {
		policy, err = signature.NewPolicyFromFile(opts.policyPath)
	}
	if err != nil {
		return nil, err
	}
	return signature.NewPolicyContext(policy)
}

// commandTimeoutContext returns a context.Context and a cancellation callback based on opts.
// The caller should usually "defer cancel()" immediately after calling this.
func (opts *globalOptions) commandTimeoutContext() (context.Context, context.CancelFunc) {
	ctx := context.Background()
	var cancel context.CancelFunc = func() {}
	if opts.commandTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, opts.commandTimeout)
	}
	return ctx, cancel
}
