package i18n_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/lonelyman0108/cfst-ddns/internal/i18n"
	"github.com/lonelyman0108/cfst-ddns/internal/notify"
	"github.com/lonelyman0108/cfst-ddns/internal/provider"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

// hasCJK 判断文案是否含中文 / 日文字符；只含品牌名、参数名、示例地址的文案无需翻译。
func hasCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			return true
		}
	}
	return false
}

// checkKeys 报告在某种语言中缺少翻译的原文。
func checkKeys(t *testing.T, what string, keys []string) {
	t.Helper()
	for _, l := range []i18n.Lang{i18n.En, i18n.ZhTW, i18n.Ja} {
		var missing []string
		for _, k := range keys {
			if hasCJK(k) && !i18n.Has(l, k) {
				missing = append(missing, strconv.Quote(k))
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			t.Errorf("%s: %s 缺少 %d 条翻译:\n%s", what, l, len(missing), strings.Join(missing, "\n"))
		}
	}
}

// TestSchemaCoverage 确保全部服务商与通知渠道元数据中的中文文案都有翻译。
func TestSchemaCoverage(t *testing.T) {
	var keys []string
	for _, metas := range [][]schema.TypeMeta{provider.Metas(), notify.Metas()} {
		if len(metas) == 0 {
			t.Fatal("注册表为空")
		}
		for _, m := range metas {
			keys = append(keys, m.Texts()...)
		}
	}
	checkKeys(t, "schema", keys)
}

// extraKeys 为不经由 i18n 调用直接书写、由接口层按语言翻译的原文。
var extraKeys = []string{
	// cfst.MirrorPresets
	"直连 GitHub",
	// legacy 导入的默认名称
	"Cloudflare（v1 导入）", "DNSPod（v1 导入）", "Bark（v1 导入）", "Telegram（v1 导入）", "v1 导入任务",
}

// TestSourceCoverage 扫描源码中 i18n.New / Errorf / M / T / Sprintf / Text 以及 failMsg、badFile、warn
// 的字符串字面量参数，确保都有翻译。
func TestSourceCoverage(t *testing.T) {
	funcs := map[string]bool{"New": true, "Errorf": true, "M": true, "T": true, "Sprintf": true, "Text": true}
	local := map[string]bool{"failMsg": true, "badFile": true, "warn": true}
	keys := append([]string{}, extraKeys...)
	fset := token.NewFileSet()
	root := filepath.Join("..")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") ||
			filepath.Base(filepath.Dir(path)) == "i18n" {
			return err
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			match := false
			switch fn := call.Fun.(type) {
			case *ast.SelectorExpr:
				if x, ok := fn.X.(*ast.Ident); ok && x.Name == "i18n" {
					match = funcs[fn.Sel.Name]
				} else {
					match = local[fn.Sel.Name]
				}
			case *ast.Ident:
				match = local[fn.Name]
			}
			if !match {
				return true
			}
			for _, a := range call.Args {
				if lit, ok := a.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if s, err := strconv.Unquote(lit.Value); err == nil {
						keys = append(keys, s)
					}
					break
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) < 50 {
		t.Fatalf("只找到 %d 条原文，扫描规则可能失效", len(keys))
	}
	checkKeys(t, "source", keys)
}
