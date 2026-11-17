package local

import (
	"bytes"
	"context"
	"strings"
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
`

func TestLocalProvider(t *testing.T) {
	var buf bytes.Buffer
	prov := NewLocalProvider(sampleDiff, &buf)

	pr, err := prov.GetPullRequest(context.Background(), "local", "repo", 1)
	if err != nil {
		t.Fatalf("GetPullRequest failed: %v", err)
	}

	diffFiles, err := prov.GetDiff(context.Background(), pr)
	if err != nil {
		t.Fatalf("GetDiff failed: %v", err)
	}
	if len(diffFiles) != 1 {
		t.Fatalf("expected 1 diff file, got %d", len(diffFiles))
	}

	err = prov.PublishComment(context.Background(), pr, "Everything looks good.")
	if err != nil {
		t.Fatalf("PublishComment failed: %v", err)
	}
	if !strings.Contains(buf.String(), "Everything looks good.") {
		t.Errorf("expected buffer to contain comment")
	}

	findings := []domain.ReviewFinding{
		{
			File:          "main.go",
			StartLine:     5,
			EndLine:       7,
			IssueTitle:    "Print function",
			Description:   "fmt.Println is preferred",
			Severity:      domain.SeverityLow,
			ExistingCode:  "println()",
			SuggestedCode: "fmt.Println()",
		},
	}

	err = prov.PublishInlineComments(context.Background(), pr, findings)
	if err != nil {
		t.Fatalf("PublishInlineComments failed: %v", err)
	}
	if !strings.Contains(buf.String(), "Print function") {
		t.Errorf("expected buffer to contain inline finding")
	}
}
