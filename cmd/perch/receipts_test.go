package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"perch/internal/launch"
)

func TestReceiptsOfflineJSONAndDestinationFilter(t *testing.T) {
	j := &launch.Journal{Dir: filepath.Join(t.TempDir(), "receipts")}
	at := time.Now()
	s := launch.Submission{Destination: "unreachable.invalid", Name: "test", Started: at, Receipt: launch.Receipt{At: at, Token: "perch-000000000000000000000001"}}
	if err := j.Save(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runReceipts([]string{"--host", "unreachable.invalid", "--json"}, &out, j); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Source  string
		Entries []struct {
			Status  string
			Receipt launch.Receipt
		}
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Source != "local_submission_journal" || len(result.Entries) != 1 || result.Entries[0].Status != "unconfirmed" || result.Entries[0].Receipt.Token != s.Receipt.Token {
		t.Fatal(out.String())
	}
	out.Reset()
	if err := runReceipts([]string{"--local", "--json"}, &out, j); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"entries":[]`) {
		t.Fatal(out.String())
	}
	for _, args := range [][]string{{"--local", "--host", "bad"}, {"42"}, {"--submit"}} {
		if err := runReceipts(args, io.Discard, j); err == nil {
			t.Fatal("accepted", args)
		}
	}
}
