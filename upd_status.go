package updaterini

type CheckStatus int

const (
	CheckSuccess   CheckStatus = iota // no errors
	CheckHasErrors                    // process finished successfully, but with errors
	CheckFailure                      // process interrupted by error
)

type SourceStatus struct {
	Source UpdateSource // link to source instance, cast it to base class
	Errors []error      // source errors
	Status CheckStatus  // source status
}

func (ss *SourceStatus) appendError(err error, critical bool) {
	ss.Errors = append(ss.Errors, err)
	if ss.Status == CheckFailure || critical {
		ss.Status = CheckFailure
	} else {
		ss.Status = CheckHasErrors
	}
}

type SourceCheckStatus struct {
	SourcesStatuses []SourceStatus // sources statuses
	Status          CheckStatus    // sources check status
}

func (scs *SourceCheckStatus) updateSourceCheckStatus() {
	hasSuccess := false
	for _, source := range scs.SourcesStatuses {
		if source.Status != CheckFailure {
			hasSuccess = true
		}
		if scs.Status < source.Status {
			scs.Status = source.Status
		}
	}
	if scs.Status == CheckFailure && hasSuccess {
		scs.Status = CheckHasErrors
	}
}
