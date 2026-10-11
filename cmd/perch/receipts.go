package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"perch/internal/launch"
)

func runReceipts(args []string, out io.Writer, journal *launch.Journal) error {
	flags := flag.NewFlagSet("perch receipts", flag.ContinueOnError)
	flags.SetOutput(out)
	host := flags.String("host", "", "filter by the exact recorded SSH destination")
	local := flags.Bool("local", false, "show only submissions made through local Slurm")
	asJSON := flags.Bool("json", false, "print versioned local receipt records")
	flags.Usage = func() {
		fmt.Fprint(out, "Saved submission receipts. Offline; never contacts Slurm or resubmits.\n\nUsage: perch receipts [--host HOST | --local] [--json]\n\n")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if len(flags.Args()) != 0 {
		return fmt.Errorf("receipts takes no positional arguments")
	}
	if *local && *host != "" {
		return fmt.Errorf("choose --host or --local")
	}
	if *local {
		*host = "local"
	}
	if journal == nil {
		var err error
		journal, err = launch.DefaultJournal()
		if err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	entries, err := journal.List(ctx, *host)
	if err != nil {
		return err
	}
	type record struct {
		launch.Submission
		Status string `json:"status"`
	}
	if *asJSON {
		records := make([]record, 0, len(entries))
		for _, entry := range entries {
			records = append(records, record{entry, entry.Status()})
		}
		return json.NewEncoder(out).Encode(struct {
			Version int      `json:"schema_version"`
			Source  string   `json:"source"`
			Entries []record `json:"entries"`
		}{1, "local_submission_journal", records})
	}
	if _, err = fmt.Fprintln(out, "Perch · saved submissions\n\nLocal receipts, not current job states. Destinations are recorded SSH arguments."); err != nil {
		return err
	}
	if len(entries) == 0 {
		_, err = fmt.Fprintln(out, "\nNo saved submissions match. Receipts start with submissions made by this version.")
		return err
	}
	for _, entry := range entries {
		id := entry.Receipt.JobID
		if id == "" {
			id = "no confirmed job ID"
		}
		if _, err = fmt.Fprintf(out, "\n%s · %s · %s\n%s · %s · %s\nComment: %s\n", entry.Started.Local().Format("2006-01-02 15:04:05"), entry.Status(), id, entry.Destination, entry.Name, entry.Receipt.Cluster, entry.Receipt.Token); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(out, "\nAccepted means Slurm returned an ID, not that the job ran.\nUnconfirmed requests may not have reached Slurm, or may have been accepted.\nCheck squeue/sacct comments for the token before submitting again.\nNo commands, scripts, logs or credentials are stored in this journal.")
	return err
}
