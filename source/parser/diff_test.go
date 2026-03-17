package parser

import (
	"testing"
)

const sampleDiff = `diff --git a/main.go b/main.go
index abc1234..def5678 100644
--- a/main.go
+++ b/main.go
@@ -1,5 +1,6 @@
 package main
 
 import "fmt"
+import "os"
 
 func main() {
@@ -10,3 +11,5 @@ func main() {
 	fmt.Println("hello")
+	name := os.Args[1]
+	fmt.Println("Hello,", name)
 }
diff --git a/utils/helper.go b/utils/helper.go
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/utils/helper.go
@@ -0,0 +1,8 @@
+package utils
+
+import "strings"
+
+// Capitalize capitalizes the first letter.
+func Capitalize(s string) string {
+	return strings.ToUpper(s[:1]) + s[1:]
+}
`

func TestParseUnifiedDiff(t *testing.T) {
	files := ParseUnifiedDiff(sampleDiff)

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	// First file: main.go
	if files[0].Path != "main.go" {
		t.Errorf("expected path 'main.go', got '%s'", files[0].Path)
	}
	if len(files[0].Hunks) != 2 {
		t.Fatalf("expected 2 hunks in main.go, got %d", len(files[0].Hunks))
	}

	// Second file: utils/helper.go
	if files[1].Path != "utils/helper.go" {
		t.Errorf("expected path 'utils/helper.go', got '%s'", files[1].Path)
	}
	if len(files[1].Hunks) != 1 {
		t.Fatalf("expected 1 hunk in helper.go, got %d", len(files[1].Hunks))
	}

	// Check added lines count in helper.go
	addedCount := 0
	for _, l := range files[1].Hunks[0].Lines {
		if l.Type == "add" {
			addedCount++
		}
	}
	if addedCount != 8 {
		t.Errorf("expected 8 added lines in helper.go, got %d", addedCount)
	}
}

func TestValidLines(t *testing.T) {
	files := ParseUnifiedDiff(sampleDiff)
	valid := ValidLines(files)

	// main.go should have added lines
	mainLines, ok := valid["main.go"]
	if !ok {
		t.Fatal("expected main.go in valid lines")
	}
	if !mainLines[4] {
		t.Error("expected line 4 to be a valid add line in main.go")
	}

	// utils/helper.go should have lines 1-8 as valid
	helperLines, ok := valid["utils/helper.go"]
	if !ok {
		t.Fatal("expected utils/helper.go in valid lines")
	}
	for i := 1; i <= 8; i++ {
		if !helperLines[i] {
			t.Errorf("expected line %d to be valid in helper.go", i)
		}
	}
}
