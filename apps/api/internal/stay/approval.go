package stay

import "time"

// PendingApprovalTTL is the same 72 hours the core invite already uses. A
// second notion of validity would be a second thing to explain to an operator
// and a second thing to get wrong.
const PendingApprovalTTL = 72 * time.Hour

// Provenance separates the stay an identified operator created from the one the
// open channel produced. It is not a stay status: the wait for approval is
// provenance plus a stamp, and core.stay_status gains no value in this feature.
type Provenance string

const (
	ProvenanceAssisted    Provenance = "assisted"
	ProvenanceSelfService Provenance = "self_service"
)

func (p Provenance) Valid() bool {
	return p == ProvenanceAssisted || p == ProvenanceSelfService
}

// ApprovalState mirrors core.stays.approval_state, with one addition the column
// cannot carry: ApprovalNotRequired stands for the SQL NULL of an assisted
// stay. Keeping the absence explicit is what lets the zero value mean "nobody
// decided yet whether this stay counts" and be refused.
type ApprovalState string

const (
	ApprovalUnset       ApprovalState = ""
	ApprovalNotRequired ApprovalState = "not_required"
	ApprovalPending     ApprovalState = "pending"
	ApprovalApproved    ApprovalState = "approved"
	ApprovalRejected    ApprovalState = "rejected"
	ApprovalExpired     ApprovalState = "expired"
)

var approvalStates = map[ApprovalState]bool{
	ApprovalNotRequired: true, ApprovalPending: true, ApprovalApproved: true,
	ApprovalRejected: true, ApprovalExpired: true,
}

func (a ApprovalState) Valid() bool {
	return approvalStates[a]
}

// Countable is the single definition of what reaches presence and publication.
// Only the assisted stay and the approved self-registration count; a pending
// one is a claim nobody has signed for yet, and neither a rejected nor an
// expired one ever becomes one.
func (a ApprovalState) Countable() bool {
	return a == ApprovalNotRequired || a == ApprovalApproved
}

// ApprovalStateFromColumn maps the nullable column, where NULL is the assisted
// stay that needs no approval at all.
func ApprovalStateFromColumn(value *string) ApprovalState {
	if value == nil {
		return ApprovalNotRequired
	}
	return ApprovalState(*value)
}

// ApprovalStateColumn is the inverse: the assisted stay writes NULL.
func ApprovalStateColumn(value ApprovalState) *string {
	if value == ApprovalNotRequired || value == ApprovalUnset {
		return nil
	}
	column := string(value)
	return &column
}

// RejectionReason is a closed list, on the precedent of
// questionnaire_change_reason_valid. platform.audit_events is append-only with
// no UPDATE and no DELETE for the application role, so free text there would
// become permanent personal data on the very path designed to erase it.
type RejectionReason string

const (
	RejectionIdentityNotVerified RejectionReason = "identity_not_verified"
	RejectionNotAGuest           RejectionReason = "not_a_guest"
	RejectionDuplicate           RejectionReason = "duplicate"
	RejectionDataIncorrect       RejectionReason = "data_incorrect"
	RejectionOther               RejectionReason = "other"
)

var rejectionReasons = map[RejectionReason]bool{
	RejectionIdentityNotVerified: true, RejectionNotAGuest: true,
	RejectionDuplicate: true, RejectionDataIncorrect: true, RejectionOther: true,
}

func (r RejectionReason) Valid() bool {
	return rejectionReasons[r]
}
