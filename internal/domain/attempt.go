package domain

// AttemptAfter is a partial order for observations of the same scoped job ID.
// Submit distinguishes reused IDs; Restarts distinguishes retries when Submit
// remains unchanged. Missing submit time or a different scope cannot establish
// that a record supersedes a previously observed attempt.
func (j Job) AttemptAfter(other Job) bool {
	if j.ID != other.ID || j.Cluster != other.Cluster || j.User != other.User {
		return false
	}
	if j.Submit.Equal(other.Submit) {
		return j.Restarts > other.Restarts
	}
	return !j.Submit.IsZero() && !other.Submit.IsZero() && j.Submit.After(other.Submit)
}
