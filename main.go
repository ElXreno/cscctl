package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ElXreno/cscctl/internal/at"
	"github.com/ElXreno/cscctl/internal/device"
)

const version = "0.1.0" // x-release-please-version

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "cscctl",
		Short:        "Samsung Galaxy CSC changer for Linux (no flash, no root, no wipe)",
		Version:      version,
		SilenceUsage: true,
	}
	root.AddCommand(infoCmd(), listCmd(), portsCmd(), setCmd(), doctorCmd())
	return root
}

type conn struct {
	port    string
	verbose bool
	timeout time.Duration
}

func addConnFlags(cmd *cobra.Command) *conn {
	c := &conn{}
	cmd.Flags().StringVar(&c.port, "port", "", "serial port (default: autodetect Samsung VID 04e8)")
	cmd.Flags().BoolVar(&c.verbose, "verbose", false, "log every AT command sent and received")
	cmd.Flags().DurationVar(&c.timeout, "timeout", 6*time.Second, "per-command response timeout")
	return c
}

func (c *conn) open() (*at.Client, error) {
	name := c.port
	if name == "" {
		detected, err := device.DetectSamsungPort()
		if err != nil {
			return nil, err
		}
		name = detected
	}
	return at.Open(name, c.timeout, c.logger())
}

func (c *conn) logger() at.Logger {
	if !c.verbose {
		return nil
	}
	return func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "  "+format+"\n", args...)
	}
}

func infoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Read device info (model, active CSC, IMEI) via AT+DEVCONINFO",
		Args:  cobra.NoArgs,
	}
	c := addConnFlags(cmd)
	cmd.RunE = func(_ *cobra.Command, _ []string) error {
		client, err := c.open()
		if err != nil {
			return err
		}
		info, err := device.NewChanger(client).ReadInfo()
		if err != nil {
			return err
		}
		printInfo(info)
		return nil
	}
	return cmd
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the CSCs bundled on the device (requires adb)",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			codes, err := device.AvailableCSCs(ctx)
			if err != nil {
				return err
			}
			sort.Strings(codes)
			fmt.Printf("%d CSCs available on device:\n", len(codes))
			for _, row := range columns(codes, 8) {
				fmt.Println(row)
			}
			return nil
		},
	}
}

func portsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ports",
		Short: "List serial ports and flag the Samsung modem",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			ports, err := device.ListPorts()
			if err != nil {
				return err
			}
			if len(ports) == 0 {
				fmt.Println("no serial ports found")
				return nil
			}
			for _, p := range ports {
				tag := ""
				if p.IsSamsung {
					tag = "  <- Samsung modem"
				}
				fmt.Printf("%-16s VID:PID %s:%s  %s%s\n", p.Name, dash(p.VID), dash(p.PID), dash(p.Product), tag)
			}
			return nil
		},
	}
}

func setCmd() *cobra.Command {
	var (
		dryRun    bool
		assumeYes bool
		reboot    bool
	)
	cmd := &cobra.Command{
		Use:   "set <CSC>",
		Short: "Change the active CSC, e.g. cscctl set XXV",
		Args:  cobra.ExactArgs(1),
	}
	c := addConnFlags(cmd)
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the AT sequence without sending it")
	cmd.Flags().BoolVar(&assumeYes, "yes", false, "skip the confirmation prompt")
	cmd.Flags().BoolVar(&reboot, "reboot", true, "reboot the device after a successful change")
	cmd.RunE = func(_ *cobra.Command, args []string) error {
		csc := strings.ToUpper(strings.TrimSpace(args[0]))
		if !device.ValidCSC(csc) {
			return fmt.Errorf("invalid CSC %q (must be 3 letters, e.g. XXV)", csc)
		}
		if dryRun {
			device.PrintPlan(csc, reboot)
			return nil
		}

		client, err := c.open()
		if err != nil {
			return err
		}
		ch := device.NewChanger(client)

		before, err := ch.ReadInfo()
		if err != nil {
			fmt.Fprintln(os.Stderr, "warning: could not read current device info:", err)
		} else {
			printInfo(before)
			if before.CSC == csc {
				fmt.Printf("device is already on CSC %s; nothing to do\n", csc)
				return nil
			}
		}

		if !assumeYes {
			fmt.Printf("\nChange active CSC %s -> %s on %s? [y/N] ", dash(before.CSC), csc, client.Name())
			if !confirm() {
				return fmt.Errorf("aborted")
			}
		}

		if err := ch.SetCSC(csc); err != nil {
			return err
		}
		fmt.Printf("CSC set to %s\n", csc)

		if reboot {
			fmt.Println("rebooting device...")
			if err := ch.Reboot(); err != nil {
				fmt.Fprintln(os.Stderr, "note: auto-reboot may not have applied, reboot manually:", err)
			}
		} else {
			fmt.Println("skipping reboot; reboot manually to finalize the change")
		}
		fmt.Printf("Done. After reboot, 'cscctl info' should show %s.\n", csc)
		return nil
	}
	return cmd
}

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run preflight checks and print the on-phone setup steps",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if port, err := device.DetectSamsungPort(); err == nil {
				fmt.Printf("[ok] Samsung serial port: %s\n", port)
			} else {
				fmt.Printf("[--] %v\n", err)
			}
			if inGroup("dialout") {
				fmt.Println("[ok] user is in the 'dialout' group")
			} else {
				fmt.Println("[--] user is not in 'dialout' (serial access needs it, or run with sudo)")
			}
			if _, err := exec.LookPath("adb"); err == nil {
				fmt.Println("[ok] adb found (enables 'cscctl list')")
			} else {
				fmt.Println("[--] adb not found (optional, used by 'cscctl list')")
			}
			fmt.Println()
			fmt.Println("Setup: enable USB debugging and connect over USB. If the modem port is missing")
			fmt.Println("or 'set' is rejected, switch USB mode to File transfer (MTP), then enable")
			fmt.Println("Developer options -> '3GPP AT commands' and dial *#0*#.")
			return nil
		},
	}
}

func confirm() bool {
	var resp string
	if _, err := fmt.Scanln(&resp); err != nil {
		return false
	}
	resp = strings.ToLower(strings.TrimSpace(resp))
	return resp == "y" || resp == "yes"
}

func inGroup(name string) bool {
	u, err := user.Current()
	if err != nil {
		return false
	}
	g, err := user.LookupGroup(name)
	if err != nil {
		return false
	}
	gids, err := u.GroupIds()
	if err != nil {
		return false
	}
	for _, gid := range gids {
		if gid == g.Gid {
			return true
		}
	}
	return false
}

func printInfo(info device.Info) {
	fmt.Println("Device info:")
	fmt.Printf("  Model:      %s\n", dash(info.Model))
	fmt.Printf("  Active CSC: %s\n", dash(info.CSC))
	fmt.Printf("  OMC code:   %s\n", dash(info.OMCCode))
	fmt.Printf("  Serial:     %s\n", dash(info.Serial))
	fmt.Printf("  IMEI:       %s\n", dash(info.IMEI))
	if info.SWVer != "" {
		fmt.Printf("  SW version: %s\n", info.SWVer)
	}
}

func dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func columns(items []string, perRow int) []string {
	var rows []string
	for i := 0; i < len(items); i += perRow {
		end := i + perRow
		if end > len(items) {
			end = len(items)
		}
		rows = append(rows, "  "+strings.Join(items[i:end], "  "))
	}
	return rows
}
