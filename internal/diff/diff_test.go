package diff

import (
	"testing"

	"pr-review-go/internal/domain"
)

const sampleDiff = `diff --git a/main.go b/main.go
index 83db48f..bf269f4 100644
--- a/main.go
+++ b/main.go
@@ -1,5 +1,7 @@
 package main
 
+import "fmt"
+
 func main() {
-	println("hello")
+	fmt.Println("hello world")
 }
diff --git a/go.sum b/go.sum
new file mode 100644
index 0000000..e69de29
--- /dev/null
+++ b/go.sum
@@ -0,0 +1,2 @@
+github.com/foo/bar v1.0.0 h1:abc=
+github.com/foo/bar v1.0.0/go.mod h1:xyz=
`

func TestParseUnifiedDiff(t *testing.T) {
	files, err := ParseUnifiedDiff(sampleDiff)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff failed: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	if files[0].DisplayPath() != "main.go" {
		t.Errorf("got first file %q, want main.go", files[0].DisplayPath())
	}
	if files[1].DisplayPath() != "go.sum" {
		t.Errorf("got second file %q, want go.sum", files[1].DisplayPath())
	}
	if !files[1].IsNew {
		t.Errorf("expected go.sum to be marked as new file")
	}
}

func TestFilterFiles(t *testing.T) {
	files, err := ParseUnifiedDiff(sampleDiff)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff failed: %v", err)
	}

	patterns := []string{"go.sum", "*.lock"}
	filtered := FilterFiles(files, patterns)

	if len(filtered) != 1 {
		t.Fatalf("expected 1 file after filter, got %d", len(filtered))
	}
	if filtered[0].DisplayPath() != "main.go" {
		t.Errorf("got %q, want main.go", filtered[0].DisplayPath())
	}
}

func TestChunkFiles(t *testing.T) {
	files := []domain.DiffFile{
		{NewPath: "file1.go", Patch: "patch 1 with some lines of code"},
		{NewPath: "file2.go", Patch: "patch 2 with some more lines of code"},
		{NewPath: "file3.go", Patch: "patch 3 with even more lines of code"},
	}

	// High token budget: all in 1 chunk
	chunks := ChunkFiles(files, 5000)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk with high budget, got %d", len(chunks))
	}

	// Small budget: each file in separate chunk
	chunks = ChunkFiles(files, 35)
	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunks with small budget, got %d", len(chunks))
	}
}
