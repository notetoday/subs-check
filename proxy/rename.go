package proxies

import (
	"fmt"
	"strings"
	"sync"

	"github.com/biter777/countries"
)

var (
	counter     = make(map[string]int)
	counterLock = sync.Mutex{}
)

// Rename 用"地区中文名 + 两位序号"重命名节点,如 "香港01"、"美国02"。
// 地区查询失败或代码无法识别时统一用 "备用" 兜底,如 "备用03"。
// 计数器按原始地区码分桶,同一地区的节点序号连续递增,保证名字唯一。
func Rename(name string) string {
	counterLock.Lock()
	defer counterLock.Unlock()

	counter[name]++
	return RegionDisplayName(name) + fmt.Sprintf("%02d", counter[name])
}

// ResetRenameCounter 将所有计数器重置为 0
func ResetRenameCounter() {
	counterLock.Lock()
	defer counterLock.Unlock()

	counter = make(map[string]int)
}

// RegionDisplayName 把 IP 库返回的两位地区码转成展示名:
//   - 已收录 → 中文名(如 "US" → "美国")
//   - 未收录但合法 → 原码原样展示(如 "IS" → "IS")
//   - 空/无法识别 → "备用"(地区查询失败的兜底,不再使用 ❓Other)
func RegionDisplayName(name string) string {
	code := strings.ToUpper(strings.TrimSpace(name))
	switch c := countries.ByName(code); c {
	case countries.Unknown, countries.None:
		return "备用"
	}
	if cn, ok := regionCN[code]; ok {
		return cn
	}
	return code
}

// regionCN 常见国家/地区代码(IP 库返回的两位字母码)的中文名对照表。
var regionCN = map[string]string{
	"US": "美国", "CA": "加拿大", "MX": "墨西哥", "BR": "巴西", "AR": "阿根廷",
	"CL": "智利", "CO": "哥伦比亚", "PE": "秘鲁", "UY": "乌拉圭", "PY": "巴拉圭",
	"BO": "玻利维亚", "EC": "厄瓜多尔", "VE": "委内瑞拉", "CR": "哥斯达黎加", "PA": "巴拿马",
	"CU": "古巴", "DO": "多米尼加", "GT": "危地马拉", "HN": "洪都拉斯", "NI": "尼加拉瓜",
	"SV": "萨尔瓦多", "JM": "牙买加", "TT": "特立尼达", "BS": "巴哈马", "BB": "巴巴多斯",
	"BZ": "伯利兹", "GY": "圭亚那", "SR": "苏里南",
	"GB": "英国", "IE": "爱尔兰", "FR": "法国", "DE": "德国", "NL": "荷兰",
	"BE": "比利时", "LU": "卢森堡", "CH": "瑞士", "AT": "奥地利", "IT": "意大利",
	"ES": "西班牙", "PT": "葡萄牙", "GR": "希腊", "DK": "丹麦", "SE": "瑞典",
	"NO": "挪威", "FI": "芬兰", "IS": "冰岛", "PL": "波兰", "CZ": "捷克",
	"SK": "斯洛伐克", "HU": "匈牙利", "RO": "罗马尼亚", "BG": "保加利亚", "HR": "克罗地亚",
	"SI": "斯洛文尼亚", "RS": "塞尔维亚", "BA": "波黑", "MK": "北马其顿", "AL": "阿尔巴尼亚",
	"ME": "黑山", "EE": "爱沙尼亚", "LV": "拉脱维亚", "LT": "立陶宛", "BY": "白俄罗斯",
	"UA": "乌克兰", "MD": "摩尔多瓦", "RU": "俄罗斯", "TR": "土耳其", "CY": "塞浦路斯",
	"MT": "马耳他", "GI": "直布罗陀",
	"JP": "日本", "KR": "韩国", "CN": "中国", "HK": "香港", "MO": "澳门",
	"TW": "台湾", "MN": "蒙古", "SG": "新加坡", "MY": "马来西亚", "TH": "泰国",
	"VN": "越南", "PH": "菲律宾", "ID": "印尼", "KH": "柬埔寨", "LA": "老挝",
	"MM": "缅甸", "BN": "文莱", "IN": "印度", "PK": "巴基斯坦", "BD": "孟加拉国",
	"LK": "斯里兰卡", "NP": "尼泊尔", "BT": "不丹", "MV": "马尔代夫", "AF": "阿富汗",
	"KZ": "哈萨克斯坦", "UZ": "乌兹别克斯坦", "TM": "土库曼斯坦", "KG": "吉尔吉斯斯坦", "TJ": "塔吉克斯坦",
	"AE": "阿联酋", "SA": "沙特阿拉伯", "QA": "卡塔尔", "KW": "科威特", "BH": "巴林",
	"OM": "阿曼", "YE": "也门", "IQ": "伊拉克", "IR": "伊朗", "SY": "叙利亚",
	"JO": "约旦", "LB": "黎巴嫩", "IL": "以色列", "PS": "巴勒斯坦", "GE": "格鲁吉亚",
	"AM": "亚美尼亚", "AZ": "阿塞拜疆",
	"EG": "埃及", "LY": "利比亚", "TN": "突尼斯", "DZ": "阿尔及利亚", "MA": "摩洛哥",
	"SD": "苏丹", "ET": "埃塞俄比亚", "KE": "肯尼亚", "TZ": "坦桑尼亚", "UG": "乌干达",
	"RW": "卢旺达", "NG": "尼日利亚", "GH": "加纳", "CI": "科特迪瓦", "SN": "塞内加尔",
	"CM": "喀麦隆", "AO": "安哥拉", "ZA": "南非", "ZM": "赞比亚", "ZW": "津巴布韦",
	"MZ": "莫桑比克",
	"AU": "澳大利亚", "NZ": "新西兰", "FJ": "斐济", "PG": "巴布亚新几内亚",
}
