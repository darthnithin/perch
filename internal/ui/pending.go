package ui

import (
	"perch/internal/domain"
	"time"
)

func pendingEstimate(j domain.Job, now time.Time) (string, string) {
	if j.EstimatedStart.IsZero() {
		return "No start estimate reported", ""
	}
	format := "02 Jan 15:04"
	if j.EstimatedStart.Local().Year() != now.Local().Year() {
		format = "02 Jan 2006 15:04"
	}
	when := j.EstimatedStart.Local().Format(format)
	if !j.EstimatedStart.After(now) {
		return "Past estimate: " + when, "Still queued; awaiting an updated scheduler estimate."
	}
	return "Tentative start: " + when, "Scheduler estimate; timing can change."
}

// These describe the reported scheduler reason, not every possible blocker or
// an estimate of queue order. Keep the original reason visible in the inspector.
// Source: https://slurm.schedmd.com/job_reason_codes.html
func pendingMessage(reason string) (string, string) {
	switch reason {
	case "JobHeldUser":
		return "Held by user or coordinator", "A user or account coordinator placed this job on hold. Review the hold before releasing it."
	case "JobHeldAdmin":
		return "Held by administrator", "An administrator placed this job on hold. Ask cluster support about the hold before resubmitting."
	case "InvalidAccount":
		return "Account needs attention", "Slurm rejected the job's account. Check the submitted script and your account access."
	case "InvalidQOS":
		return "Quality of service needs attention", "Slurm reports an invalid quality of service (QoS). Check the submitted script and allowed QoS settings."
	case "PartitionDown", "PartitionInactive":
		return "Partition is not starting jobs", "The requested partition cannot start jobs in its current state. Check Partitions for the reported state."
	case "PartitionTimeLimit":
		return "Requested time exceeds partition limit", "Compare the submitted time limit with the partition's current limit before preparing a corrected job."
	case "BadConstraints":
		return "Requested constraints cannot be met", "Slurm cannot satisfy this job's requirements. Review the submitted constraints before resubmitting."
	case "ReqNodeNotAvail":
		return "Required node is unavailable", "A required node is unavailable. It may be busy, reserved or out of service; inspect Nodes for its reported state."
	case "BeginTime":
		return "Waiting for its earliest start", "The job's requested earliest start time has not arrived. It may still wait for resources after that time."
	case "Dependency":
		return "Waiting for prerequisites", "A dependency on another job has not been satisfied yet."
	case "DependencyNeverSatisfied":
		return "Prerequisite cannot succeed", "Review the prerequisite outcome before submitting again."
	case "JobArrayTaskLimit":
		return "Waiting for an array slot", "This array has reached its limit of simultaneous tasks."
	case "Priority":
		return "Waiting for priority", "Higher-priority work exists in this partition or reservation."
	case "Resources":
		return "Waiting for resources", "The requested resources are not available yet."
	default:
		return "Queued", ""
	}
}

// Unknown reasons remain visible in the bounded preview. The full inspector
// retains the original reason without assigning it an invented explanation.
func pendingSummary(reason string) string {
	title, detail := pendingMessage(reason)
	if detail == "" && reason != "" && reason != "None" {
		return title + " · " + domain.Label(reason)
	}
	return title
}
