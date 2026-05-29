package device

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ElXreno/cscctl/internal/at"
)

var UnlockSequence = []string{
	"AT+KSTRINGB=0,3",
	"AT+DUMPCTRL=1,0",
	"AT+DEBUGLVC=0,5",
	"AT+SWATD=0",
	"AT+ACTIVATE=0,0,0",
	"AT+SWATD=1",
	"AT+DEBUGLVC=0,5",
}

var (
	fieldPattern = regexp.MustCompile(`([A-Z0-9_]+)\(([^)]*)\)`)
	cscPattern   = regexp.MustCompile(`^[A-Z]{3}$`)
)

type Info struct {
	Model   string
	CSC     string
	OMCCode string
	Serial  string
	IMEI    string
	SWVer   string
	Fields  map[string]string
}

func ValidCSC(csc string) bool { return cscPattern.MatchString(csc) }

func ParseInfo(resp at.Response) (Info, error) {
	info := Info{Fields: map[string]string{}}
	matches := fieldPattern.FindAllStringSubmatch(resp.Raw, -1)
	if len(matches) == 0 {
		return info, fmt.Errorf("no DEVCONINFO fields in response: %q", resp.Text())
	}
	for _, m := range matches {
		info.Fields[m[1]] = strings.TrimSpace(m[2])
	}
	info.Model = first(info.Fields, "MN", "MODEL", "MODEL_NAME")
	info.CSC = first(info.Fields, "PRD", "SALESCODE", "CSC")
	info.Serial = first(info.Fields, "SN", "SERIAL")
	info.IMEI = first(info.Fields, "IMEI")
	info.SWVer = first(info.Fields, "VER", "SW_VER", "SWVER")
	info.OMCCode = first(info.Fields, "OMCCODE", "OMC")
	if info.OMCCode == "" {
		info.OMCCode = omcFromVersion(info.Model, info.SWVer)
	}
	return info, nil
}

func omcFromVersion(model, ver string) string {
	parts := strings.Split(ver, "/")
	if len(parts) < 2 {
		return ""
	}
	rest := strings.TrimPrefix(parts[1], strings.TrimPrefix(model, "SM-"))
	if len(rest) >= 3 && cscPattern.MatchString(rest[:3]) {
		return rest[:3]
	}
	return ""
}

func first(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := m[k]; v != "" {
			return v
		}
	}
	return ""
}

type Changer struct {
	client *at.Client
	pause  time.Duration
}

func NewChanger(client *at.Client) *Changer {
	return &Changer{client: client, pause: 300 * time.Millisecond}
}

func (c *Changer) ReadInfo() (Info, error) {
	resp, _ := c.client.Send("AT+DEVCONINFO")
	return ParseInfo(resp)
}

func (c *Changer) Unlock() {
	c.client.SendAll(UnlockSequence, c.pause)
}

func (c *Changer) SetCSC(csc string) error {
	if !ValidCSC(csc) {
		return fmt.Errorf("invalid CSC %q (must be 3 uppercase letters)", csc)
	}
	c.Unlock()
	for _, cmd := range []string{"AT+PRECONFG=2," + csc, "AT+PRECONF=2," + csc} {
		resp, err := c.client.Send(cmd)
		if err == nil && resp.OK {
			return nil
		}
		time.Sleep(c.pause)
	}
	return fmt.Errorf("device did not accept CSC %s (tried PRECONFG and PRECONF)", csc)
}

func (c *Changer) Reboot() error {
	if _, err := c.client.Send("AT+SWATD=0"); err != nil {
		return err
	}
	time.Sleep(c.pause)
	_, err := c.client.Send("AT+CFUN=1,1")
	return err
}

func PrintPlan(csc string, reboot bool) {
	fmt.Printf("Plan to set active CSC to %s (no data wipe):\n", csc)
	for _, cmd := range UnlockSequence {
		fmt.Printf("  %s\n", cmd)
	}
	fmt.Printf("  AT+PRECONFG=2,%s   (fallback: AT+PRECONF=2,%s)\n", csc, csc)
	if reboot {
		fmt.Println("  AT+SWATD=0")
		fmt.Println("  AT+CFUN=1,1")
	}
}
