package schema

import "testing"

var meta = TypeMeta{Type: "x", Fields: []Field{
	{Key: "mode", Label: "模式", Type: Select, Default: "a", Options: []Option{{Value: "a"}, {Value: "b"}}},
	{Key: "token", Label: "令牌", Type: Password, Required: true, Secret: true, ShowIf: &Condition{Key: "mode", Value: "a"}},
	{Key: "port", Label: "端口", Type: Number},
}}

func TestValidate(t *testing.T) {
	if err := meta.Validate(meta.ApplyDefaults(Config{})); err == nil {
		t.Fatal("缺少必填项应报错")
	}
	if err := meta.Validate(Config{"mode": "b"}); err != nil {
		t.Fatalf("隐藏字段不应校验: %v", err)
	}
	if err := meta.Validate(Config{"mode": "c"}); err == nil {
		t.Fatal("非法选项应报错")
	}
	if err := meta.Validate(Config{"mode": "b", "port": "abc"}); err == nil {
		t.Fatal("非数字应报错")
	}
}

func TestMaskAndMerge(t *testing.T) {
	old := Config{"mode": "a", "token": "secret", "port": "1"}
	masked := meta.Masked(old)
	if masked["token"] != Mask || old["token"] != "secret" {
		t.Fatalf("masked = %v", masked)
	}
	merged := meta.Merge(old, Config{"mode": "a", "token": Mask, "port": "2", "junk": "x"})
	if merged["token"] != "secret" || merged["port"] != "2" || merged["junk"] != "" {
		t.Fatalf("merged = %v", merged)
	}
	merged = meta.Merge(old, Config{"mode": "a", "token": "new"})
	if merged["token"] != "new" {
		t.Fatalf("merged = %v", merged)
	}
}
