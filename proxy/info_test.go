package proxies

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetPing0_Parse 验证 ping0.cc/geo 4行纯文本响应能解析出国家代码和IP
func TestGetPing0_Parse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := `216.152.147.28
加拿大 安大略省 多伦多
AS62563
GTHost`
		w.Write([]byte(body))
	}))
	defer ts.Close()

	// 临时替换 URL，直接通过 httptest server 模拟
	old := ping0URL
	ping0URL = ts.URL
	defer func() { ping0URL = old }()

	loc, ip := GetPing0(ts.Client())
	if loc != "CA" {
		t.Errorf("loc = %q, want CA", loc)
	}
	if ip != "216.152.147.28" {
		t.Errorf("ip = %q, want 216.152.147.28", ip)
	}
}

// TestGetPing0_UnknownCountry 验证未映射的中文国家名返回空 loc
func TestGetPing0_UnknownCountry(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := `1.2.3.4
亚特兰蒂斯 某省 某市
AS999
Org`
		w.Write([]byte(body))
	}))
	defer ts.Close()

	old := ping0URL
	ping0URL = ts.URL
	defer func() { ping0URL = old }()

	loc, ip := GetPing0(ts.Client())
	if loc != "" {
		t.Errorf("loc = %q, want empty", loc)
	}
	if ip != "1.2.3.4" {
		t.Errorf("ip = %q, want 1.2.3.4", ip)
	}
}

// TestGetPing0_ShortResponse 验证少于2行的响应返回空
func TestGetPing0_ShortResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("1.2.3.4"))
	}))
	defer ts.Close()

	old := ping0URL
	ping0URL = ts.URL
	defer func() { ping0URL = old }()

	loc, ip := GetPing0(ts.Client())
	if loc != "" || ip != "" {
		t.Errorf("loc=%q ip=%q, want both empty", loc, ip)
	}
}

// TestGetPing0_Non200 验证非200状态码返回空
func TestGetPing0_Non200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "blocked", http.StatusForbidden)
	}))
	defer ts.Close()

	old := ping0URL
	ping0URL = ts.URL
	defer func() { ping0URL = old }()

	loc, ip := GetPing0(ts.Client())
	if loc != "" || ip != "" {
		t.Errorf("loc=%q ip=%q, want both empty", loc, ip)
	}
}
