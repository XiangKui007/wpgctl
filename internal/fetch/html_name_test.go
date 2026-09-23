package fetch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeHTMLDirName(t *testing.T) {
	cases := map[string]string{
		"3.outwork.zip":              "outwork",
		"4.outworkApp":               "outworkApp",
		"alarm.v4.8.1":               "alarm",
		"alarm.v4.8.1.zip":           "alarm",
		"device-plat.v4.8.1_patch.1": "device-plat",
		"gis-manage_v4.2.1":          "gis-manage",
		"gisPatrolWeb_v4.2.1":        "gisPatrolWeb",
		"graphSystem.v4.8.1":         "graphSystem",
		"ops.v4.8.1":                 "ops",
		"report-center.v4.7.6":       "report-center",
		"report-conf-ys.v4.7.6":      "report-conf-ys",
		"smallPortal.v4.8.1":         "smallPortal",
		"userAdmin.v4.8.1":           "userAdmin",
		"waterwork-app":              "waterwork-app",
		"water-work-front":           "water-work-front",
		"3.alarm.v4.8.1":             "alarm",
	}
	for in, want := range cases {
		if got := NormalizeHTMLDirName(in); got != want {
			t.Errorf("%q → %q, want %q", in, got, want)
		}
	}
}

func TestExpandHTMLZipStripsVersionAndPrefix(t *testing.T) {
	root := t.TempDir()
	html := filepath.Join(root, "html")
	if err := os.MkdirAll(html, 0o755); err != nil {
		t.Fatal(err)
	}

	inner := filepath.Join(t.TempDir(), "pack")
	if err := os.MkdirAll(filepath.Join(inner, "alarm.v4.8.1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inner, "alarm.v4.8.1", "index.html"), []byte("alarm"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := zipDir(filepath.Join(html, "alarm.v4.8.1.zip"), inner, ""); err != nil {
		t.Fatal(err)
	}

	outwork := filepath.Join(t.TempDir(), "ow")
	if err := os.MkdirAll(outwork, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outwork, "index.html"), []byte("outwork"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := zipDir(filepath.Join(html, "3.outwork.zip"), outwork, ""); err != nil {
		t.Fatal(err)
	}

	res, err := Expand(ExpandOptions{Root: html, DestName: NormalizeHTMLDirName})
	if err != nil {
		t.Fatal(err)
	}
	if res.ZipExtracted != 2 {
		t.Fatalf("extracted=%d steps=%v", res.ZipExtracted, res.Steps)
	}
	if !fileExists(filepath.Join(html, "alarm", "index.html")) {
		t.Fatalf("want html/alarm/index.html, steps=%v", res.Steps)
	}
	if fileExists(filepath.Join(html, "alarm.v4.8.1", "index.html")) {
		t.Fatal("versioned alarm dir should not remain as extract target")
	}
	if !fileExists(filepath.Join(html, "outwork", "index.html")) {
		t.Fatal("want html/outwork/index.html")
	}
	if fileExists(filepath.Join(html, "3.outwork", "index.html")) {
		t.Fatal("3.outwork dir should not be used")
	}
}

func TestFlattenHTMLDistAndNestedVersion(t *testing.T) {
	html := filepath.Join(t.TempDir(), "html")
	nested := filepath.Join(html, "userAdmin.v4.8.1", "userAdmin.v4.8.1", "dist")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	normalizeHTMLTree(html, nil)
	if !fileExists(filepath.Join(html, "userAdmin", "index.html")) {
		t.Fatal("want html/userAdmin/index.html after flatten")
	}
}

// 研发把 dist 再套一层 front/ 打包：html/alarm/front/dist/index.html 要掀成 html/alarm/index.html。
func TestFlattenHTMLFrontWrapper(t *testing.T) {
	html := filepath.Join(t.TempDir(), "html")
	nested := filepath.Join(html, "alarm", "front", "dist")
	if err := os.MkdirAll(filepath.Join(nested, "static"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	normalizeHTMLTree(html, nil)
	if !fileExists(filepath.Join(html, "alarm", "index.html")) {
		t.Fatal("want html/alarm/index.html after flattening front/dist")
	}
	if fileExists(filepath.Join(html, "alarm", "front")) {
		t.Fatal("front/ wrapper should be removed")
	}
	if st, err := os.Stat(filepath.Join(html, "alarm", "static")); err != nil || !st.IsDir() {
		t.Fatal("static/ should be lifted alongside index.html")
	}
}

func TestRenameHTMLExtractDirs(t *testing.T) {
	html := filepath.Join(t.TempDir(), "html")
	old := filepath.Join(html, "gis-manage_v4.2.1")
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "index.html"), []byte("gis"), 0o644); err != nil {
		t.Fatal(err)
	}
	renameHTMLExtractDirs(html, nil)
	if !fileExists(filepath.Join(html, "gis-manage", "index.html")) {
		t.Fatal("renamed dir missing")
	}
	if fileExists(filepath.Join(html, "gis-manage_v4.2.1", "index.html")) {
		t.Fatal("old versioned dir should be gone")
	}
}
