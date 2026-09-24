package engine

import (
	"github.com/lonelyman0108/cfst-ddns/internal/provider"
)

// Update 表示把一条现有记录改为新值。
type Update struct {
	Record   provider.Record
	NewValue string
}

// Plan 为一组同名同类型记录的同步方案。
type Plan struct {
	Keep    []provider.Record
	Updates []Update
	Creates []string
	Deletes []provider.Record
}

// Changed 判断方案是否会修改 DNS。
func (p Plan) Changed() bool {
	return len(p.Updates)+len(p.Creates)+len(p.Deletes) > 0
}

// MakePlan 计算把 existing 同步为 desired（去重、有序）所需的最少操作：
// 已匹配的记录保留；未匹配的旧记录优先改写为未匹配的新值；不足则新建，多余则删除。
func MakePlan(existing []provider.Record, desired []string) Plan {
	var p Plan
	want := map[string]bool{}
	var wantOrder []string
	for _, v := range desired {
		if v != "" && !want[v] {
			want[v] = true
			wantOrder = append(wantOrder, v)
		}
	}
	matched := map[string]bool{}
	var spare []provider.Record
	for _, r := range existing {
		if want[r.Value] && !matched[r.Value] {
			matched[r.Value] = true
			p.Keep = append(p.Keep, r)
		} else {
			spare = append(spare, r)
		}
	}
	var missing []string
	for _, v := range wantOrder {
		if !matched[v] {
			missing = append(missing, v)
		}
	}
	for len(spare) > 0 && len(missing) > 0 {
		p.Updates = append(p.Updates, Update{Record: spare[0], NewValue: missing[0]})
		spare, missing = spare[1:], missing[1:]
	}
	if len(missing) > 0 {
		p.Creates = missing
	}
	if len(spare) > 0 {
		p.Deletes = spare
	}
	return p
}

// defaultLines 为各服务商"默认线路"的表示方式。
var defaultLines = map[string]bool{"": true, "默认": true, "default": true, "default_view": true, "0": true}

// sameLine 判断记录是否属于目标线路。
func sameLine(recordLine, targetLine string) bool {
	if targetLine == "" || defaultLines[targetLine] {
		return defaultLines[recordLine]
	}
	return recordLine == targetLine
}
