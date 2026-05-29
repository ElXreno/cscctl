# cscctl

Change the region code (**CSC**) on Samsung Galaxy phones from Linux - no Windows, no SamFW, no root, and no data wipe.

![license](https://img.shields.io/badge/license-MIT-blue)
![go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go&logoColor=white)
![platform](https://img.shields.io/badge/platform-Linux-444)

Samsung phones carry a **CSC** (Consumer Software Customization) code that selects region features, network/IMS profiles, and which OTA track the device follows. Region-locked features such as native call recording sit behind it. `cscctl` flips the *active* CSC in place over the phone's USB modem interface - the same thing Windows tools like SamFW do, but open-source, scriptable, and native to Linux.

> [!WARNING]
> Changing your CSC is unofficial and unsupported by Samsung. It does not trip Knox and does not wipe data, but switching regions swaps your network/IMS profiles - always test calls and VoLTE afterwards, and be ready to revert. Use at your own risk.

## Features

- **No flashing, no wipe, no root** - just AT commands over the modem port; Knox stays `0x0`.
- **Autodetects** the Samsung modem port (USB VID `04e8`).
- **`info`** shows model, active CSC, OMC group and IMEI; **`list`** shows the CSCs your firmware actually bundles.
- **`--dry-run`** prints the exact AT sequence without sending anything.
- **Safe by default** - confirmation prompt before any change, full `--verbose` command log.
- Single static binary, packaged as a **Nix flake**.

## Tested on

| Device | Model | Change |
| --- | --- | --- |
| Galaxy S25 Ultra | SM-S938B | `EUX` -> `XXV` |
| Galaxy S23 | SM-S911B | `EUX` -> `XXV` |

Both via the no-flash AT method (no `3GPP AT commands` toggle or `*#0*#` dialer code needed); the change persisted after reboot.

## Requirements

- Linux, with your user in the `dialout` group for serial access (or run as root).
- A Samsung Galaxy phone connected over USB with **USB debugging** enabled.
- [`adb`](https://developer.android.com/tools/adb) - only needed for `cscctl list`.
- [Nix](https://nixos.org) (recommended), or Go 1.26+ to build from source.

## Installation

### NixOS

Add the flake as an input and pull in the package, plus the bits cscctl needs
(`dialout` for serial access, and adb for `cscctl list`):

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    cscctl = {
      url = "github:ElXreno/cscctl";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { nixpkgs, cscctl, ... }:
    {
      nixosConfigurations."hostname" = nixpkgs.lib.nixosSystem {
        ...
        modules = [
          ({ pkgs, ... }: {
            environment.systemPackages = [
              cscctl.packages.${pkgs.stdenv.hostPlatform.system}.default
              pkgs.android-tools # adb, only needed for 'cscctl list'
            ];

            # serial access to the phone's modem port
            users.users."yourname".extraGroups = [ "dialout" ];
          })
        ];
      };
    };
}
```

### Run without installing

```console
nix run github:ElXreno/cscctl -- doctor   # run without installing
nix build github:ElXreno/cscctl           # build ./result/bin/cscctl
```

### From source

```console
git clone https://github.com/ElXreno/cscctl
cd cscctl
go build -o cscctl .
```

## Quick start

```console
cscctl doctor              # check host setup and the connection
cscctl info                # see your current region (active CSC)
cscctl list                # which CSCs can you switch to? (needs adb)
cscctl set XXV --dry-run   # preview the exact AT sequence
cscctl set XXV             # change it (asks to confirm, then reboots)
# ...the phone reboots...
cscctl info                # confirm the new CSC
```

Popular targets: `XXV` (Vietnam) and `INS` (India) enable native call recording. You can only switch to a CSC that `cscctl list` reports - i.e. one bundled in your firmware's multi-CSC group.

## Usage

```console
Samsung Galaxy CSC changer for Linux (no flash, no root, no wipe)

Usage:
  cscctl [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  doctor      Run preflight checks and print the on-phone setup steps
  help        Help about any command
  info        Read device info (model, active CSC, IMEI) via AT+DEVCONINFO
  list        List the CSCs bundled on the device (requires adb)
  ports       List serial ports and flag the Samsung modem
  set         Change the active CSC, e.g. cscctl set XXV

Flags:
  -h, --help      help for cscctl
  -v, --version   version for cscctl

Use "cscctl [command] --help" for more information about a command.
```

Flags for `set` (`cscctl set --help`):

```console
Change the active CSC, e.g. cscctl set XXV

Usage:
  cscctl set <CSC> [flags]

Flags:
      --dry-run            print the AT sequence without sending it
  -h, --help               help for set
      --port string        serial port (default: autodetect Samsung VID 04e8)
      --reboot             reboot the device after a successful change (default true)
      --timeout duration   per-command response timeout (default 6s)
      --verbose            log every AT command sent and received
      --yes                skip the confirmation prompt
```

## On-phone setup

1. Enable **USB debugging** (Developer options), connect over USB, and authorize the host.
2. The modem port shows up automatically - `cscctl` unlocks the AT interface itself.

If the port never appears or `set` is rejected, switch USB mode to **File transfer (MTP)**, and as a last resort enable **3GPP AT commands** in Developer options and dial **`*#0*#`**.

## How it works

`set` opens the modem serial port and sends, with pacing and `\r`-terminated lines:

```text
AT+KSTRINGB=0,3
AT+DUMPCTRL=1,0
AT+DEBUGLVC=0,5
AT+SWATD=0
AT+ACTIVATE=0,0,0
AT+SWATD=1
AT+DEBUGLVC=0,5
AT+PRECONFG=2,<CSC>     (falls back to AT+PRECONF=2,<CSC>)
AT+SWATD=0
AT+CFUN=1,1            (reboot)
```

`SWATD` / `ACTIVATE` / `DUMPCTRL` unlock Samsung's protected-AT distributor, and `PRECONFG=2,<CSC>` writes the active sales code in place. The new CSC takes effect on reboot, so confirm afterwards with `cscctl info` (a pre-reboot read is unreliable because the distributor is re-locked). `PRECONFG` appears in Samsung's own `ProtectedATCommand` user-open list, so this is a sanctioned interface, not an exploit.

## Troubleshooting

- **`no Samsung serial port found`** - set USB to File transfer (MTP), replug, and confirm you are in the `dialout` group (`cscctl doctor`). Pass `--port /dev/ttyACM0` to override autodetect.
- **CSC reverts after reboot** - your firmware blocked the in-place change; flash the target region's `HOME_CSC` with [Thor](https://github.com/Samsung-Loki/Thor) or odin4 instead (still no wipe).
- **No VoLTE / Wi-Fi calling after switching** - revert with `cscctl set <OLD_CSC>`; some regions' IMS profiles do not match every carrier.
- **`cscctl list` fails** - it needs `adb` with USB debugging enabled and the host authorized.

## Limitations

- Switches only the *active* CSC, and only to codes already in your firmware's multi-CSC group (`cscctl list`). Reaching another group needs a `HOME_CSC` flash.
- Cannot change the locked factory **sales code** (the 4th octet), so on its own it does not fix OTA eligibility.

## Credits

`cscctl` is an independent, from-scratch Go implementation and includes no code from other projects. The technique it uses (the AT-command sequence is a device protocol, not anyone's source code) is documented in this prior work, credited with thanks:

- [Alephgsm/Change-CSC-AT-Command](https://github.com/Alephgsm/Change-CSC-AT-Command)
- Dmitry Khlebnikov's [modem-commands writeup](https://dmitry.khlebnikov.net/2025/10/09/samsung-csc-change-using-modem-commands/)
- [zacharee/SamsungDecryptStuff](https://github.com/zacharee/SamsungDecryptStuff) - reference `CSCChanger`

## License

[MIT](LICENSE)
