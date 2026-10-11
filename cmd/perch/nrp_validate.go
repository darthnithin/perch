package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"perch/internal/nautilus"
)

func readNRPJobManifest(filename, namespace string) ([]byte, error) {
	// Inspect first: do not wait indefinitely on a FIFO or consume stdin that an
	// explicitly requested foreground credential plugin may need for login.
	info, err := os.Stat(filename)
	if err != nil || !info.Mode().IsRegular() || info.Size() > nautilus.MaxJobManifestBytes {
		return nil, fmt.Errorf("validation needs a readable regular Job JSON file of at most 256 KiB")
	}
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("unable to open the Job JSON file")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, nautilus.MaxJobManifestBytes+1))
	if err != nil {
		return nil, fmt.Errorf("unable to read the Job JSON file")
	}
	if _, err = nautilus.InspectJobManifest(data, namespace); err != nil {
		return nil, err
	}
	return data, nil
}

func writeNRPValidation(out io.Writer, response nrpResponse, result nautilus.JobValidation, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(response); err != nil {
			return err
		}
	} else {
		var b strings.Builder
		fmt.Fprintf(&b, "Perch · Nautilus Job validation\n\n%s / %s\nContext: %s\n", result.Namespace, result.Name, response.Context)
		fmt.Fprintf(&b, "Job dry-run %s · not submitted\n", result.Status)
		if result.Error != nil {
			fmt.Fprintf(&b, "%s\n", result.Error.Message)
		}
		fmt.Fprintf(&b, "\nNext: %s\n", result.NextStep)
		if result.Quotas.OK() {
			fmt.Fprintf(&b, "\nQuota snapshot: %d ResourceQuota objects\n", len(result.Quotas.Collection.Items))
			for _, quota := range result.Quotas.Collection.Items {
				fmt.Fprintf(&b, "  %s · %s\n", quota.Name, quota.ScopeDescription())
				for _, resource := range quota.Resources {
					remaining := "unknown"
					if resource.Remaining != nil {
						remaining = *resource.Remaining
					}
					fmt.Fprintf(&b, "    %s: %s remaining in this quota\n", resource.Name, remaining)
				}
			}
		} else {
			b.WriteString("\nQuota snapshot unavailable; Job admission does not prove Pod capacity.\n")
		}
		fmt.Fprintf(&b, "\nManifest SHA-256: %s\n\n", result.ManifestSHA256)
		for _, note := range result.Notes {
			fmt.Fprintf(&b, "• %s\n", note)
		}
		if err := writeTerminalText(out, b.String()); err != nil {
			return err
		}
	}
	if !result.Admitted {
		return fmt.Errorf("Job dry-run %s; no workload was submitted", result.Status)
	}
	return nil
}
