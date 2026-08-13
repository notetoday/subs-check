package proxies

import (
	"fmt"
	"strings"
	"sync"
)

var (
	counter     = make(map[string]int)
	counterLock = sync.Mutex{}
)

func Rename(name string) string {
	counterLock.Lock()
	defer counterLock.Unlock()

	// 先转换展示名,再对最终名称计数,避免用原始国家码计数、用中文名读取导致序号恒为 0
	if zh, ok := zhCountryAlpha2Reversed[strings.ToUpper(name)]; ok {
		name = zh
	} else if name == "" {
		// 国家未知(如 IP 查询全部失败)时的兜底展示名
		name = "备用"
	}
	counter[name]++
	return fmt.Sprintf("%s_%02d", name, counter[name])
}

// ResetRenameCounter 将所有计数器重置为 0
func ResetRenameCounter() {
	counterLock.Lock()
	defer counterLock.Unlock()

	counter = make(map[string]int)
}
