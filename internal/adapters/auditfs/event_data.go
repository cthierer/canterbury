package auditfs

import (
	"fmt"
	"strings"
	"time"

	"github.com/cthierer/canterbury/internal/domain/audit"
	"github.com/cthierer/canterbury/internal/domain/vault"
)

const schemaVersion int32 = 1

type actorData struct {
	Issuer      string   `json:"issuer"`
	SubjectHash string   `json:"subject_hash"`
	Scopes      []string `json:"scopes"`
}

func actorDataFromDomain(actor audit.Actor) (actorData, error) {
	scopes, err := stringsFromScopes(actor.Scopes)
	if err != nil {
		return actorData{}, fmt.Errorf("build actor scopes: %w", err)
	}

	return actorData{
		Issuer:      actor.Issuer,
		SubjectHash: actor.SubjectHash,
		Scopes:      scopes,
	}, nil
}

type clientData struct {
	Interface         string `json:"interface"`
	UserAgent         string `json:"user_agent"`
	RemoteAddressHash string `json:"remote_addr_hash,omitempty"`
}

func clientDataFromDomain(client audit.Client) clientData {
	return clientData{
		Interface:         client.Interface.String(),
		UserAgent:         client.UserAgent,
		RemoteAddressHash: client.RemoteAddressHash,
	}
}

type policyData struct {
	MappingChecksum string   `json:"mapping_checksum,omitempty"`
	MatchedScopes   []string `json:"matched_scopes"`
	Decision        string   `json:"decision"`
}

func policyDataFromDomain(policy audit.Policy) (policyData, error) {
	matchedScopes, err := stringsFromScopes(policy.MatchedScopes)
	if err != nil {
		return policyData{}, fmt.Errorf("build policy matched scopes: %w", err)
	}

	return policyData{
		MappingChecksum: policy.MappingChecksum,
		MatchedScopes:   matchedScopes,
		Decision:        policy.Decision.String(),
	}, nil
}

type outcomeData struct {
	Status   string        `json:"status"`
	Code     string        `json:"code"`
	Duration time.Duration `json:"duration_ns"`
}

func outcomeDataFromDomain(outcome audit.Outcome) outcomeData {
	return outcomeData{
		Status:   outcome.Status.String(),
		Code:     outcome.Code.String(),
		Duration: outcome.Duration,
	}
}

type eventData struct {
	ID            string             `json:"id"`
	SchemaVersion int32              `json:"schema_version"`
	OccurredAt    time.Time          `json:"occurred_at"`
	EventType     string             `json:"event_type"`
	RequestID     string             `json:"request_id,omitempty"`
	TraceID       string             `json:"trace_id,omitempty"`
	Actor         actorData          `json:"actor"`
	Client        clientData         `json:"client"`
	Policy        policyData         `json:"policy"`
	Outcome       outcomeData        `json:"outcome"`
	Details       audit.EventDetails `json:"details"`
}

func eventDataFromDomain(event audit.Event) (eventData, error) {
	eventID := strings.TrimSpace(event.ID.String())
	if eventID == "" {
		return eventData{}, ErrEventMissingID
	}

	timestamp := event.OccurredAt.UTC()
	if timestamp.IsZero() {
		return eventData{}, ErrEventMissingTimestamp
	}

	eventType, err := eventTypeFromDomain(event.Type())
	if err != nil {
		return eventData{}, fmt.Errorf("format event type: %w", err)
	}

	actor, err := actorDataFromDomain(event.Actor)
	if err != nil {
		return eventData{}, fmt.Errorf("format actor data: %w", err)
	}

	policy, err := policyDataFromDomain(event.Policy)
	if err != nil {
		return eventData{}, fmt.Errorf("format policy data: %w", err)
	}

	client := clientDataFromDomain(event.Client)
	outcome := outcomeDataFromDomain(event.Outcome)

	return eventData{
		ID:            eventID,
		SchemaVersion: schemaVersion,
		OccurredAt:    timestamp,
		EventType:     eventType,
		RequestID:     event.RequestID.String(),
		TraceID:       event.TraceID.String(),
		Actor:         actor,
		Client:        client,
		Policy:        policy,
		Outcome:       outcome,
		Details:       event.Details,
	}, nil
}

func eventTypeFromDomain(eventType audit.EventType) (string, error) {
	if eventType == audit.EventTypeUnknown {
		return "", ErrEventUnknownType
	}

	normalized := strings.TrimSpace(eventType.String())
	if normalized == "" {
		return "", ErrEventUnknownType
	}

	return normalized, nil
}

func stringsFromScopes(scopes []vault.Scope) ([]string, error) {
	stringScopes := make([]string, len(scopes))
	for i, scopeVal := range scopes {
		normalized := strings.TrimSpace(scopeVal.String())
		if normalized == "" {
			return nil, ErrInvalidScopes
		}

		stringScopes[i] = normalized
	}

	return stringScopes, nil
}
