package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/leporel/huetension/internal/sandbox"
)

// sandboxFlags collects the image-sandbox knobs shared by the `mcp`,
// `web`, and `serve` subcommands. Each command embeds this struct and
// binds the flags via bindSandboxFlags; the security posture they
// describe gates image.extract / POST /extract identically across
// transports, so keeping one definition avoids per-command drift.
type sandboxFlags struct {
	readOnly             bool
	root                 string
	allowHosts           []string
	maxImageBytes        int64
	blockPrivateNetworks bool
}

// bindSandboxFlags registers the five sandbox flags on cmd.
//
// pathInputLabel names the path-style input each transport rejects under
// --read-only ("/extract path inputs" for web, "image.extract path
// inputs" for mcp) so the help reads naturally per command. autoOnNote
// completes the "auto-on for …" tail on the two flags that auto-enable
// (the trigger differs: bind exposure for web/serve, transport for mcp).
func bindSandboxFlags(cmd *cobra.Command, sf *sandboxFlags, pathInputLabel, autoOnNote string) {
	cmd.Flags().BoolVar(&sf.readOnly, "read-only", false,
		"reject "+pathInputLabel+" (URL/data still allowed); auto-on for "+autoOnNote)
	cmd.Flags().StringVar(&sf.root, "root", ".",
		"directory below which "+pathInputLabel+" must resolve; empty disables filesystem access")
	cmd.Flags().StringSliceVar(&sf.allowHosts, "allow-host", nil,
		"host allowlist for image URL fetches (e.g. '*.unsplash.com'); empty allows all")
	cmd.Flags().Int64Var(&sf.maxImageBytes, "max-image-bytes", 0,
		"cap on image bytes (URL body / decoded base64 / upload); 0 → 64 MiB")
	cmd.Flags().BoolVar(&sf.blockPrivateNetworks, "block-private-networks", false,
		"refuse outbound connections to loopback/private/link-local IPs in image fetches; auto-on for "+autoOnNote)
}

// toImageSandbox converts the parsed flags into the sandbox value the
// web / serve configs carry.
func (sf sandboxFlags) toImageSandbox() sandbox.ImageSandbox {
	return sandbox.ImageSandbox{
		ReadOnly:             sf.readOnly,
		Root:                 sf.root,
		AllowHosts:           sf.allowHosts,
		MaxImageBytes:        sf.maxImageBytes,
		BlockPrivateNetworks: sf.blockPrivateNetworks,
	}
}

// applyLoopbackSandboxDefaults flips --read-only and
// --block-private-networks on for a non-loopback bind unless the operator
// set them explicitly. Used by `web` and `serve`, whose exposure is keyed
// off the bind address. `mcp` keys the same rule off its transport list
// instead (see applyAutoDefaults in mcp.go).
func applyLoopbackSandboxDefaults(cmd *cobra.Command, loopback bool, sf *sandboxFlags) {
	if loopback {
		return
	}
	if !cmd.Flags().Changed("read-only") {
		sf.readOnly = true
	}
	if !cmd.Flags().Changed("block-private-networks") {
		sf.blockPrivateNetworks = true
	}
}

// validateLoopbackRootSandbox refuses to start when a non-loopback bind
// runs --read-only=false without an explicit --root. The default "."
// (cwd) is too broad to expose; the auto-defaults would have set
// ReadOnly=true had the operator not explicitly cleared it, so this path
// is reached only when --read-only=false was passed by hand. Shared by
// `web` and `serve`.
func validateLoopbackRootSandbox(cmd *cobra.Command, loopback bool, sf *sandboxFlags) error {
	if loopback || sf.readOnly || cmd.Flags().Changed("root") {
		return nil
	}
	return fmt.Errorf("refusing to start: --read-only=false on a non-loopback bind requires an explicit --root pinning the sandbox directory; %q (cwd) is too broad", sf.root)
}
