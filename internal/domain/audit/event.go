package audit

import (
	"strings"
	"time"

	"github.com/cthierer/canterbury/internal/domain/vault"
)

const maxIDLength = 128

// Actor describes the authenticated caller recorded in an audit event.
type Actor struct {
	Issuer      string
	SubjectHash string
	Scopes      []vault.Scope
}

// ClientInterface names the protocol or interface that received a request.
type ClientInterface string

func (c ClientInterface) String() string {
	return string(c)
}

const (
	// ClientInterfaceConnectRPC identifies the Connect RPC interface.
	ClientInterfaceConnectRPC ClientInterface = "connectrpc"

	// ClientInterfaceService identifies an internal service interface.
	ClientInterfaceService ClientInterface = "service"
)

// Client describes request client metadata recorded in an audit event.
type Client struct {
	Interface         ClientInterface
	UserAgent         string
	RemoteAddressHash string
}

// PolicyDecision names an authorization decision recorded in an audit event.
type PolicyDecision string

func (p PolicyDecision) String() string {
	return string(p)
}

const (
	// PolicyDecisionAllow indicates the policy allowed the operation.
	PolicyDecisionAllow PolicyDecision = "allow"

	// PolicyDecisionDeny indicates the policy denied the operation.
	PolicyDecisionDeny PolicyDecision = "deny"
)

// Policy describes the policy inputs and decision for an audited operation.
type Policy struct {
	MappingChecksum string
	MatchedScopes   []vault.Scope
	Decision        PolicyDecision
}

// OutcomeStatus names the overall result category for an audited operation.
type OutcomeStatus string

func (o OutcomeStatus) String() string {
	return string(o)
}

const (
	// OutcomeStatusSuccess indicates the operation completed successfully.
	OutcomeStatusSuccess OutcomeStatus = "success"

	// OutcomeStatusFailed indicates the operation failed due to expected input,
	// authentication, authorization, or application conditions.
	OutcomeStatusFailed OutcomeStatus = "failed"

	// OutcomeStatusError indicates the operation failed due to an unexpected
	// system error.
	OutcomeStatusError OutcomeStatus = "error"
)

// OutcomeCode names a stable service or application result code.
type OutcomeCode string

func (o OutcomeCode) String() string {
	return string(o)
}

const (
	// OutcomeCodeOK indicates the audited operation succeeded.
	OutcomeCodeOK OutcomeCode = "ok"

	// OutcomeCodePermissionDenied indicates policy denied the operation.
	OutcomeCodePermissionDenied OutcomeCode = "permission_denied"

	// OutcomeCodeUnauthenticated indicates the request lacked usable identity.
	OutcomeCodeUnauthenticated OutcomeCode = "unauthenticated"

	// OutcomeCodeInvalidArgument indicates the request was malformed or invalid.
	OutcomeCodeInvalidArgument OutcomeCode = "invalid_argument"

	// OutcomeCodeNotFound indicates the requested resource was not found.
	OutcomeCodeNotFound OutcomeCode = "not_found"

	// OutcomeCodeUnavailable indicates a required backend was unavailable.
	OutcomeCodeUnavailable OutcomeCode = "unavailable"

	// OutcomeCodeInternal indicates an unexpected internal failure.
	OutcomeCodeInternal OutcomeCode = "internal"
)

// Outcome describes the result and duration of an audited operation.
type Outcome struct {
	Status   OutcomeStatus
	Code     OutcomeCode
	Duration time.Duration
}

// EventType names a stable audit event kind.
type EventType string

func (e EventType) String() string {
	return string(e)
}

const (
	// EventTypeUnknown indicates an event with no concrete details.
	EventTypeUnknown EventType = ""

	// EventTypeAuthFailed records an authentication failure.
	EventTypeAuthFailed EventType = "auth.failed"

	// EventTypeAuthSucceeded records a successful authentication event.
	EventTypeAuthSucceeded EventType = "auth.succeeded"

	// EventTypeVaultReadAllowed records a note read that returned data.
	EventTypeVaultReadAllowed EventType = "vault.read.allowed"

	// EventTypeVaultReadDenied records a note read denied by policy.
	EventTypeVaultReadDenied EventType = "vault.read.denied"

	// EventTypeVaultReadFailed records a note read that failed before returning
	// data.
	EventTypeVaultReadFailed EventType = "vault.read.failed"

	// EventTypeVaultSearchCompleted records a search that returned results.
	EventTypeVaultSearchCompleted EventType = "vault.search.completed"

	// EventTypeVaultSearchFailed records a search that failed before returning
	// results.
	EventTypeVaultSearchFailed EventType = "vault.search.failed"
)

// EventDetails describes the event-specific audit payload.
type EventDetails interface {
	EventType() EventType
}

// EventID uniquely identifies an audit event.
type EventID string

func (e EventID) String() string {
	return string(e)
}

// NewEventID validates and normalizes an audit event identifier.
func NewEventID(value string) (EventID, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", ErrInvalidEventID
	}

	if len(normalized) > maxIDLength {
		return "", ErrInvalidEventID
	}

	return EventID(normalized), nil
}

// RequestID correlates audit events produced by one inbound request.
type RequestID string

func (r RequestID) String() string {
	return string(r)
}

// NewRequestID validates and normalizes a request identifier.
func NewRequestID(value string) (RequestID, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", ErrInvalidRequestID
	}

	if len(normalized) > maxIDLength {
		return "", ErrInvalidRequestID
	}

	return RequestID(normalized), nil
}

// TraceID correlates audit events with a distributed trace when one exists.
type TraceID string

func (t TraceID) String() string {
	return string(t)
}

// NewTraceID validates and normalizes an optional trace identifier.
func NewTraceID(value string) (TraceID, error) {
	normalized := strings.TrimSpace(value)
	if len(normalized) > maxIDLength {
		return "", ErrInvalidTraceID
	}

	return TraceID(normalized), nil
}

// Event records one auditable operation or decision.
type Event struct {
	ID         EventID
	OccurredAt time.Time
	RequestID  RequestID
	TraceID    TraceID
	Actor      Actor
	Client     Client
	Policy     Policy
	Outcome    Outcome
	Details    EventDetails
}

// Type returns the stable event type declared by the event details.
func (e Event) Type() EventType {
	if e.Details == nil {
		return EventTypeUnknown
	}

	return e.Details.EventType()
}
