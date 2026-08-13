package proxies

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"log/slog"

	"github.com/metacubex/mihomo/common/convert"
)

type geoResult struct {
	loc string
	ip  string
}

// zhCountryAlpha2 中文国家名 -> ISO 3166-1 Alpha2 代码。
// ping0.cc/geo 返回的中文位置首字段是国家名，而 countries.ByName 不支持中文，
// 因此这里维护一份常用映射，命中失败则返回空串走兜底。
var zhCountryAlpha2 = map[string]string{
	"中国": "CN", "中国香港": "HK", "香港": "HK", "中国澳门": "MO", "澳门": "MO",
	"中国台湾": "TW", "台湾": "TW", "美国": "US", "加拿大": "CA", "日本": "JP",
	"韩国": "KR", "新加坡": "SG", "马来西亚": "MY", "泰国": "TH", "越南": "VN",
	"印度": "IN", "印度尼西亚": "ID", "菲律宾": "PH", "缅甸": "MM", "老挝": "LA",
	"柬埔寨": "KH", "蒙古": "MN", "孟加拉国": "BD", "巴基斯坦": "PK", "斯里兰卡": "LK",
	"尼泊尔": "NP", "哈萨克斯坦": "KZ", "乌兹别克斯坦": "UZ", "沙特阿拉伯": "SA",
	"阿联酋": "AE", "阿联酋联合酋长国": "AE", "卡塔尔": "QA", "科威特": "KW",
	"以色列": "IL", "土耳其": "TR", "伊朗": "IR", "伊拉克": "IQ",
	"英国": "GB", "法国": "FR", "德国": "DE", "意大利": "IT", "西班牙": "ES",
	"葡萄牙": "PT", "荷兰": "NL", "比利时": "BE", "瑞士": "CH", "奥地利": "AT",
	"瑞典": "SE", "挪威": "NO", "丹麦": "DK", "芬兰": "FI", "波兰": "PL",
	"捷克": "CZ", "斯洛伐克": "SK", "匈牙利": "HU", "罗马尼亚": "RO", "保加利亚": "BG",
	"希腊": "GR", "爱尔兰": "IE", "冰岛": "IS", "俄罗斯": "RU", "乌克兰": "UA",
	"白俄罗斯": "BY", "立陶宛": "LT", "拉脱维亚": "LV", "爱沙尼亚": "EE",
	"塞尔维亚": "RS", "克罗地亚": "HR", "斯洛文尼亚": "SI", "澳大利亚": "AU",
	"新西兰": "NZ", "巴西": "BR", "阿根廷": "AR", "智利": "CL", "秘鲁": "PE",
	"哥伦比亚": "CO", "墨西哥": "MX", "埃及": "EG", "南非": "ZA", "尼日利亚": "NG",
	"摩洛哥": "MA", "肯尼亚": "KE", "埃塞俄比亚": "ET",
}

// zhCountryAlpha2Reversed Alpha2 代码 -> 中文国家名，由 zhCountryAlpha2 反向生成。
// 用于节点命名时把 Alpha2 国家码转成中文展示名。
var zhCountryAlpha2Reversed = func() map[string]string {
	m := make(map[string]string, len(zhCountryAlpha2))
	for zh, code := range zhCountryAlpha2 {
		if _, ok := m[code]; !ok {
			m[code] = zh
		}
	}
	return m
}()

// ping0URL 便于测试替换
var ping0URL = "https://ping0.cc/geo"

// GetPing0 通过 ping0.cc/geo 获取地理位置。
// 返回纯文本4行：IP / 中文位置(国家 省 市) / ASN / 商家。
// 国家取第二行首个字段，经 zhCountryAlpha2 映射为 Alpha2 代码。
func GetPing0(httpClient *http.Client) (loc string, ip string) {
	url := ping0URL
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Debug(fmt.Sprintf("创建请求失败: %s", err))
		return
	}
	req.Header.Set("User-Agent", convert.RandUserAgent())
	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Debug(fmt.Sprintf("ping0获取节点位置失败: %s", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Debug(fmt.Sprintf("ping0返回非200状态码: %v", resp.StatusCode))
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Debug(fmt.Sprintf("ping0读取节点位置失败: %s", err))
		return
	}

	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) < 2 {
		slog.Debug(fmt.Sprintf("ping0响应格式不正确: %d 行", len(lines)))
		return
	}

	ip = strings.TrimSpace(lines[0])
	locLine := strings.TrimSpace(lines[1])
	// 位置行首字段为国家名，形如 "加拿大 安大略省 多伦多"
	parts := strings.Fields(locLine)
	if len(parts) == 0 {
		slog.Debug("ping0位置行为空")
		return
	}
	if code, ok := zhCountryAlpha2[parts[0]]; ok {
		loc = code
	}
	return
}

// 这里需要一个不限流的ipv4的非CF的API
// 因为ipv6在数据库中没有记载时会变成US。
// 不能用CF的API是因为我们要保留CF的节点（无proxyip的）
// GetProxyCountry 并行请求所有 IP 查询端点，按优先级返回最优结果
func GetProxyCountry(httpClient *http.Client) (loc string, ip string) {
	// 顺序代表优先级，索引越小质量越高
	checkers := []func(*http.Client) (string, string){
		GetPing0, GetMe, GetIpinfo, GetCFProxy, GetEdgeOneProxy,
	}

	results := make([]geoResult, len(checkers))
	var wg sync.WaitGroup

	for idx, fn := range checkers {
		wg.Add(1)
		go func(i int, f func(*http.Client) (string, string)) {
			defer wg.Done()
			l, p := f(httpClient)
			results[i] = geoResult{l, p}
		}(idx, fn)
	}

	wg.Wait()

	// 按优先级返回第一个成功的结果
	for _, res := range results {
		if res.loc != "" && res.ip != "" {
			return res.loc, res.ip
		}
	}
	return
}

// GetEdgeOneProxy 通过腾讯 EdgeOne 获取地理位置
func GetEdgeOneProxy(httpClient *http.Client) (loc string, ip string) {
	type GeoResponse struct {
		Eo struct {
			Geo struct {
				CountryCodeAlpha2 string `json:"countryCodeAlpha2"`
			} `json:"geo"`
			ClientIp string `json:"clientIp"`
		} `json:"eo"`
	}

	url := "https://functions-geolocation.edgeone.app/geo"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Debug(fmt.Sprintf("创建请求失败: %s", err))
		return
	}
	req.Header.Set("User-Agent", convert.RandUserAgent())
	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Debug(fmt.Sprintf("edgeone获取节点位置失败: %s", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Debug(fmt.Sprintf("edgeone返回非200状态码: %v", resp.StatusCode))
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Debug(fmt.Sprintf("edgeone读取节点位置失败: %s", err))
		return
	}

	var eo GeoResponse
	err = json.Unmarshal(body, &eo)
	if err != nil {
		slog.Debug(fmt.Sprintf("解析edgeone JSON 失败: %v", err))
		return
	}

	return eo.Eo.Geo.CountryCodeAlpha2, eo.Eo.ClientIp
}

// GetCFProxy 通过 Cloudflare cdn-cgi/trace 获取地理位置
// 局限：CF 节点需要 proxyip 落地才能访问套 CF 的网站，trace 返回的是 proxyip 落地位置而非节点真实出口位置
func GetCFProxy(httpClient *http.Client) (loc string, ip string) {
	url := "https://www.cloudflare.com/cdn-cgi/trace"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Debug(fmt.Sprintf("创建请求失败: %s", err))
		return
	}
	req.Header.Set("User-Agent", convert.RandUserAgent())
	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Debug(fmt.Sprintf("cf获取节点位置失败: %s", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Debug(fmt.Sprintf("cf返回非200状态码: %v", resp.StatusCode))
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Debug(fmt.Sprintf("cf读取节点位置失败: %s", err))
		return
	}

	// Parse the response text to find loc=XX
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "loc=") {
			loc = strings.TrimPrefix(line, "loc=")
		}
		if strings.HasPrefix(line, "ip=") {
			ip = strings.TrimPrefix(line, "ip=")
		}
	}
	return
}

// GetIPSB 通过 ip.sb 获取地理位置
func GetIPSB(httpClient *http.Client) (loc string, ip string) {
	type GeoIPData struct {
		IP      string `json:"ip"`
		Country string `json:"country_code"`
	}

	url := "https://api.ip.sb/geoip"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Debug(fmt.Sprintf("创建请求失败: %s", err))
		return
	}
	req.Header.Set("User-Agent", convert.RandUserAgent())
	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Debug(fmt.Sprintf("ip.sb获取节点位置失败: %s", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Debug(fmt.Sprintf("ip.sb返回非200状态码: %v", resp.StatusCode))
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Debug(fmt.Sprintf("ip.sb读取节点位置失败: %s", err))
		return
	}

	var geo GeoIPData
	err = json.Unmarshal(body, &geo)
	if err != nil {
		slog.Debug(fmt.Sprintf("解析ip.sb JSON 失败: %v", err))
		return
	}

	return geo.Country, geo.IP
}

func GetMe(httpClient *http.Client) (loc string, ip string) {
	type GeoIPData struct {
		IP      string `json:"ip"`
		Country string `json:"country_code"`
	}

	url := "https://ip.122911.xyz/api/ipinfo"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Debug(fmt.Sprintf("创建请求失败: %s", err))
		return
	}
	req.Header.Set("User-Agent", "subs-check (https://github.com/beck-8/subs-check)")
	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Debug(fmt.Sprintf("me获取节点位置失败: %s", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Debug(fmt.Sprintf("me返回非200状态码: %v", resp.StatusCode))
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Debug(fmt.Sprintf("me读取节点位置失败: %s", err))
		return
	}

	var geo GeoIPData
	err = json.Unmarshal(body, &geo)
	if err != nil {
		slog.Debug(fmt.Sprintf("解析me JSON 失败: %v", err))
		return
	}

	return geo.Country, geo.IP
}

func GetIpinfo(httpClient *http.Client) (loc string, ip string) {
	type GeoIPData struct {
		IP      string `json:"ip"`
		Country string `json:"country_code"`
	}

	url := string([]byte{104, 116, 116, 112, 115, 58, 47, 47, 97, 112, 105, 46, 105, 112,
		105, 110, 102, 111, 46, 105, 111, 47, 108, 105, 116, 101, 47, 109, 101, 63, 116,
		111, 107, 101, 110, 61, 48, 57, 48, 102, 54, 54, 55, 55, 57, 55, 51, 51, 98, 102})
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Debug(fmt.Sprintf("创建请求失败: %s", err))
		return
	}
	req.Header.Set("User-Agent", "subs-check (https://github.com/beck-8/subs-check)")
	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Debug(fmt.Sprintf("Ipinfo获取节点位置失败: %s", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Debug(fmt.Sprintf("Ipinfo返回非200状态码: %v", resp.StatusCode))
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Debug(fmt.Sprintf("Ipinfo读取节点位置失败: %s", err))
		return
	}

	var geo GeoIPData
	err = json.Unmarshal(body, &geo)
	if err != nil {
		slog.Debug(fmt.Sprintf("解析Ipinfo JSON 失败: %v", err))
		return
	}

	return geo.Country, geo.IP
}
