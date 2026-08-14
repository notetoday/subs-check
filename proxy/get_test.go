package proxies

import "testing"

func TestResolveSubUrlProxy_Prefix(t *testing.T) {
	useProxy, url := resolveSubUrlProxy("px:https://xin.riyuexing.dynv6.net/Ex/XingYue")
	if !useProxy {
		t.Errorf("px: 前缀应返回 useProxy=true")
	}
	want := "https://xin.riyuexing.dynv6.net/Ex/XingYue"
	if url != want {
		t.Errorf("px: 前缀应被剥离, got %q, want %q", url, want)
	}
}

func TestResolveSubUrlProxy_NoPrefix(t *testing.T) {
	useProxy, url := resolveSubUrlProxy("https://raw.githubusercontent.com/example/sub.txt")
	if useProxy {
		t.Errorf("无前缀应返回 useProxy=false")
	}
	if url != "https://raw.githubusercontent.com/example/sub.txt" {
		t.Errorf("无前缀不应改动 URL, got %q", url)
	}
}

func TestResolveSubUrlProxy_OnlyPrefix(t *testing.T) {
	useProxy, url := resolveSubUrlProxy("px:")
	if !useProxy {
		t.Errorf("px: 单独出现也应返回 useProxy=true")
	}
	if url != "" {
		t.Errorf("px: 单独出现剥离后应为空, got %q", url)
	}
}
