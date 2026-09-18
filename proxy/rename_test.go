package proxies

import "testing"

func TestRegionDisplayName(t *testing.T) {
	cases := map[string]string{
		"US":   "美国",
		"us":   "美国",
		" HK ": "香港",
		"SG":   "新加坡",
		"XX":   "备用", // 非法地区码
		"":     "备用", // 查询失败
		"   ":  "备用",
	}
	for in, want := range cases {
		if got := RegionDisplayName(in); got != want {
			t.Errorf("RegionDisplayName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRename_FormatAndCounter(t *testing.T) {
	ResetRenameCounter()
	defer ResetRenameCounter()

	if got := Rename("US"); got != "美国01" {
		t.Errorf("Rename(US) = %q, want 美国01", got)
	}
	if got := Rename("US"); got != "美国02" {
		t.Errorf("Rename(US) again = %q, want 美国02", got)
	}
	if got := Rename(""); got != "备用01" {
		t.Errorf("Rename(empty) = %q, want 备用01", got)
	}
	if got := Rename(""); got != "备用02" {
		t.Errorf("Rename(empty) again = %q, want 备用02", got)
	}
}
