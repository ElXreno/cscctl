package device

import (
	"slices"
	"testing"

	"github.com/ElXreno/cscctl/internal/at"
)

func TestParseInfoRealDevice(t *testing.T) {
	raw := "AT+DEVCONINFO\r\n+DEVCONINFO: MN(SM-S938B);BASE(UNKNOWN);" +
		"VER(S938BXXU0AAAA/S938BOXM0AAAA/S938BXXU0AAAA/S938BXXU0AAAA);" +
		"HIDVER(S938BXXU0AAAA/S938BOXM0AAAA/S938BXXU0AAAA/S938BXXU0AAAA);" +
		"MNC(00,);MCC(000,);PRD(EUX);AID();CC(NL);OMCCODE();SN(R0000000000);" +
		"IMEI(000000000000000);UN(00000000000000000000);PN(,);CON(AT,MTP);" +
		"LOCK(NONE);LIMIT(FALSE);SDP(RUNTIME);HVID(Data:196609)\r\n\r\nOK\r\n"

	info, err := ParseInfo(at.Response{Raw: raw})
	if err != nil {
		t.Fatalf("ParseInfo: %v", err)
	}
	for _, c := range []struct{ name, got, want string }{
		{"model", info.Model, "SM-S938B"},
		{"csc", info.CSC, "EUX"},
		{"omc", info.OMCCode, "OXM"},
		{"serial", info.Serial, "R0000000000"},
		{"imei", info.IMEI, "000000000000000"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

func TestParseCSCList(t *testing.T) {
	raw := "single/ACR\nsingle/EUX\nsingle/XXV\nsingle/CAU\nsingle/EUX\njunk\nsingle/TOOLONG\n"
	got := parseCSCList(raw)
	for _, want := range []string{"ACR", "EUX", "XXV", "CAU"} {
		if !slices.Contains(got, want) {
			t.Errorf("expected %q in %v", want, got)
		}
	}
	if slices.Contains(got, "TOOLONG") {
		t.Errorf("did not expect TOOLONG in %v", got)
	}
	if n := len(got); n != 4 {
		t.Errorf("expected 4 unique codes, got %d: %v", n, got)
	}
}
