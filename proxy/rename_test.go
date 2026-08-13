package proxies

import (
	"strings"
	"testing"
)

func TestRename_SameCountrySequential(t *testing.T) {
	ResetRenameCounter()
	got := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		got = append(got, Rename("US"))
	}
	want := []string{"美国_01", "美国_02", "美国_03"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Rename() 第%d次 = %q, want %q", i+1, got[i], want[i])
		}
	}
}

func TestRename_MixedCountries(t *testing.T) {
	ResetRenameCounter()
	if got := Rename("HK"); got != "香港_01" {
		t.Errorf("Rename(HK) = %q, want 香港_01", got)
	}
	if got := Rename("US"); got != "美国_01" {
		t.Errorf("Rename(US) = %q, want 美国_01", got)
	}
	if got := Rename("HK"); got != "香港_02" {
		t.Errorf("Rename(HK) 第二次 = %q, want 香港_02", got)
	}
}

func TestRename_EmptyCountryFallback(t *testing.T) {
	ResetRenameCounter()
	if got := Rename(""); got != "备用_01" {
		t.Errorf("Rename(空) = %q, want 备用_01", got)
	}
	if got := Rename(""); got != "备用_02" {
		t.Errorf("Rename(空) 第二次 = %q, want 备用_02", got)
	}
}

func TestRename_NoEmoji(t *testing.T) {
	ResetRenameCounter()
	for _, code := range []string{"US", "JP", "FR"} {
		if got := Rename(code); strings.ContainsAny(got, "🇺🇸🇯🇵🇫🇷") {
			t.Errorf("Rename(%s) = %q, 不应包含国旗 emoji", code, got)
		}
	}
}
