package filter

import "testing"

// hit 表示 path 被 exclude 规则命中（Allowed=false）
func hit(m *Matcher, path string, isDir bool) bool {
	return !m.Allowed(path, isDir)
}

func TestExcludeBasics(t *testing.T) {
	m := NewMust([]string{"*.tmp", "Thumbs.db"}, nil, false)
	cases := []struct {
		path string
		dir  bool
		want bool // want=true → 应命中
	}{
		{"a.tmp", false, true},
		{"sub/a.tmp", false, true}, // 无斜杠规则命中任意层级
		{"a.txt", false, false},
		{"sub/Thumbs.db", false, true},
		{"sub/photo.txt", false, false},
		{"dir.tmp", true, true},
	}
	for _, c := range cases {
		if got := hit(m, c.path, c.dir); got != c.want {
			t.Errorf("hit(%q,%v)=%v want %v", c.path, c.dir, got, c.want)
		}
	}
}

func TestExcludeDirSubtree(t *testing.T) {
	m := NewMust([]string{"node_modules/"}, nil, false)
	if !hit(m, "node_modules", true) {
		t.Fatal("目录 node_modules 应被排除")
	}
	if !hit(m, "a/node_modules", true) {
		t.Fatal("嵌套 node_modules 应被排除")
	}
	if !hit(m, "a/node_modules/x.js", false) {
		t.Fatal("被排除目录的子文件应视为不允许")
	}
	if hit(m, "nodemodules2/x.js", false) {
		t.Fatal("不应误伤相似名字")
	}
	if hit(m, "a/node_modulesx/y.js", false) {
		t.Fatal("前缀相似目录不应误伤")
	}
}

func TestDoubleStar(t *testing.T) {
	m := NewMust([]string{"docs/**/*.md"}, nil, false)
	if !hit(m, "docs/a.md", false) {
		t.Error("docs/a.md 应命中 docs/**/*.md（** 可为零段）")
	}
	if !hit(m, "docs/sub/a.md", false) {
		t.Error("docs/sub/a.md 应命中")
	}
	if !hit(m, "docs/x/y/z.md", false) {
		t.Error("多层子目录应命中")
	}
	if hit(m, "other/a.md", false) {
		t.Error("other/a.md 不应命中")
	}
	if hit(m, "docs/a.txt", false) {
		t.Error("非 .md 不应命中")
	}
	m2 := NewMust([]string{"**/logs/"}, nil, false)
	if !hit(m2, "a/b/logs", true) {
		t.Error("**/logs/ 应命中任意层级 logs 目录")
	}
	if hit(m2, "a/blogs", true) {
		t.Error("blogs 不应命中 **/logs/")
	}
	m3 := NewMust([]string{"archive/**"}, nil, false)
	if !hit(m3, "archive/x/y.bin", false) {
		t.Error("archive/** 应命中其下全部内容")
	}
	if hit(m3, "archived/x.bin", false) {
		t.Error("archived 不应命中 archive/**")
	}
}

func TestIncludeList(t *testing.T) {
	m := NewMust(nil, []string{"*.jpg", "*.png"}, false)
	if hit(m, "a.jpg", false) {
		t.Error("include 命中的文件应放行")
	}
	if !hit(m, "a.txt", false) {
		t.Error("include 未命中的文件应被过滤")
	}
	if hit(m, "sub", true) {
		t.Error("目录应始终放行以进入子层")
	}
	m2 := NewMust(nil, []string{"photos/"}, false)
	if hit(m2, "photos/x.jpg", false) {
		t.Error("photos/ 子文件应放行")
	}
	if !hit(m2, "other/x.jpg", false) {
		t.Error("other/ 下应被过滤")
	}
}

func TestCaseSensitive(t *testing.T) {
	mi := NewMust([]string{"*.TMP"}, nil, false)
	if !hit(mi, "a.tmp", false) {
		t.Error("大小写不敏感时 a.tmp 应被排除")
	}
	ms := NewMust([]string{"*.TMP"}, nil, true)
	if hit(ms, "a.tmp", false) {
		t.Error("大小写敏感时 a.tmp 不应被排除")
	}
	if !hit(ms, "a.TMP", false) {
		t.Error("a.TMP 应被排除")
	}
}

func TestBuiltInExcludes(t *testing.T) {
	m := NewMust(nil, nil, false)
	if !hit(m, ".filesync-trash/x", false) {
		t.Error("内置备份目录必须排除")
	}
	if !hit(m, "sub/.filesync-trash/a/b", false) {
		t.Error("嵌套内置目录必须排除")
	}
	if !hit(m, ".filesync/state.json", false) {
		t.Error("内置状态目录必须排除")
	}
	if hit(m, "filesync-trash/x", false) {
		t.Error("相似名不应误伤")
	}
}

func TestSpecialChars(t *testing.T) {
	m := NewMust([]string{"a?c.txt", "b[1].txt"}, nil, false)
	if !hit(m, "abc.txt", false) {
		t.Error("? 应匹配单字符：abc.txt 应命中 a?c.txt")
	}
	if hit(m, "abbc.txt", false) {
		t.Error("? 只匹配一个字符：abbc.txt 不应命中")
	}
	if !hit(m, "b[1].txt", false) {
		t.Error("正则元字符应按字面处理")
	}
	if hit(m, "b1.txt", false) {
		t.Error("[1] 应按字面而非字符类处理")
	}
}

func TestInvalidPattern(t *testing.T) {
	if _, err := New([]string{"/"}, nil, false); err == nil {
		t.Error("空模式应报错")
	}
}
