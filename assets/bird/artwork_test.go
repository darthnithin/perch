package bird

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"
)

// The original detailed artwork must survive terminal experiments unchanged.
func TestDetailedArtworkMatchesApprovedSource(t *testing.T) {
	manifest, err := os.ReadFile("artwork.sha256")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(manifest)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatal("invalid artwork checksum record")
		}
		data, err := os.ReadFile(fields[1])
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != fields[0] {
			t.Errorf("approved artwork changed: %s", fields[1])
		}
		count++
	}
	if count != 21 {
		t.Fatalf("expected 21 preserved images, got %d", count)
	}
}
