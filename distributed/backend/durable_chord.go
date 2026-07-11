package backend

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/songzhibin97/gkit/distributed/task"
)

const (
	DurableChordDeliveryKeyMeta = "gkit.chord_delivery_key"
	DurableChordMemberMeta      = "gkit.chord_member"
	DurableChordMemberOrdinal   = "gkit.chord_member_ordinal"

	DefaultChordRetentionSeconds int64 = 3600
)

var (
	ErrDurableChordUnsupported        = errors.New("durable chord backend unsupported")
	ErrChordInvalidInput              = errors.New("invalid group callback")
	ErrChordRegistrationConflict      = errors.New("chord registration conflict")
	ErrChordRegistrationAborted       = errors.New("chord registration aborted")
	ErrChordRegistrationOwnershipLost = errors.New("chord registration ownership lost")
	ErrChordPublicationStarted        = errors.New("chord publication started")
	ErrChordLeaseLost                 = errors.New("chord publication lease lost")
	ErrChordReceiptConflict           = errors.New("chord member receipt conflict")
	ErrChordTerminalConflict          = errors.New("chord terminal outcome conflict")
	ErrChordNotFound                  = errors.New("chord delivery not found")
)

type ChordMemberState string

const (
	ChordMemberSetup          ChordMemberState = "MEMBER_SETUP"
	ChordMemberReady          ChordMemberState = "MEMBER_READY"
	ChordMemberLeased         ChordMemberState = "MEMBER_LEASED"
	ChordMemberPublished      ChordMemberState = "MEMBER_PUBLISHED"
	ChordMemberPublishUnknown ChordMemberState = "MEMBER_PUBLISH_UNKNOWN"
	ChordMemberTerminal       ChordMemberState = "MEMBER_TERMINAL"
)

type ChordCallbackState string

const (
	ChordWaiting        ChordCallbackState = "WAITING"
	ChordReady          ChordCallbackState = "READY"
	ChordLeased         ChordCallbackState = "LEASED"
	ChordPublished      ChordCallbackState = "PUBLISHED"
	ChordPublishUnknown ChordCallbackState = "PUBLISH_UNKNOWN"
	ChordDelivered      ChordCallbackState = "DELIVERED"
	ChordSuppressed     ChordCallbackState = "SUPPRESSED"
)

type MemberTerminalOutcome string

const (
	MemberTerminalSuccess MemberTerminalOutcome = "SUCCESS"
	MemberTerminalFailure MemberTerminalOutcome = "FAILURE"
)

type CallbackTerminalOutcome string

const (
	CallbackTerminalSuccess CallbackTerminalOutcome = "SUCCESS"
	CallbackTerminalFailure CallbackTerminalOutcome = "FAILURE"
)

type ChordPublishOutcomeKind string

const (
	ChordPublishOutcomeSucceeded ChordPublishOutcomeKind = "PUBLISHED"
	ChordPublishOutcomeUnknown   ChordPublishOutcomeKind = "UNKNOWN"
	ChordPublishOutcomeRejected  ChordPublishOutcomeKind = "REJECTED"
)

type ChordMemberRegistration struct {
	Ordinal int    `json:"ordinal" bson:"ordinal"`
	TaskID  string `json:"task_id" bson:"task_id"`
	Payload []byte `json:"payload" bson:"payload"`
}

type ChordRegistration struct {
	DeliveryKey    string                    `json:"delivery_key" bson:"delivery_key"`
	DefinitionHash string                    `json:"definition_hash" bson:"definition_hash"`
	GroupID        string                    `json:"group_id" bson:"group_id"`
	GroupName      string                    `json:"group_name" bson:"group_name"`
	Retention      int64                     `json:"retention" bson:"retention"`
	Callback       []byte                    `json:"callback" bson:"callback"`
	Members        []ChordMemberRegistration `json:"members" bson:"members"`
}

type ChordRegistrationRef struct {
	DeliveryKey string `json:"delivery_key" bson:"delivery_key"`
	Owner       string `json:"owner" bson:"owner"`
	Version     int64  `json:"version" bson:"version"`
	Created     bool   `json:"created" bson:"created"`
}

type ChordMemberReceipt struct {
	TaskID  string                `json:"task_id" bson:"task_id"`
	Outcome MemberTerminalOutcome `json:"outcome" bson:"outcome"`
	Results []*task.Result        `json:"results" bson:"results"`
}

type ChordMemberPublication struct {
	Ordinal              int                 `json:"ordinal" bson:"ordinal"`
	TaskID               string              `json:"task_id" bson:"task_id"`
	Payload              []byte              `json:"payload" bson:"payload"`
	State                ChordMemberState    `json:"state" bson:"state"`
	Version              int64               `json:"version" bson:"version"`
	LeaseOwner           string              `json:"lease_owner,omitempty" bson:"lease_owner,omitempty"`
	LeaseExpiresAt       time.Time           `json:"lease_expires_at,omitempty" bson:"lease_expires_at,omitempty"`
	Attempts             int                 `json:"attempts" bson:"attempts"`
	NextAttemptAt        time.Time           `json:"next_attempt_at,omitempty" bson:"next_attempt_at,omitempty"`
	ConfirmationDeadline time.Time           `json:"confirmation_deadline,omitempty" bson:"confirmation_deadline,omitempty"`
	LastError            string              `json:"last_error,omitempty" bson:"last_error,omitempty"`
	Receipt              *ChordMemberReceipt `json:"receipt,omitempty" bson:"receipt,omitempty"`
}

type ChordDelivery struct {
	DeliveryKey              string                   `json:"delivery_key" bson:"delivery_key"`
	DefinitionHash           string                   `json:"definition_hash" bson:"definition_hash"`
	GroupID                  string                   `json:"group_id" bson:"group_id"`
	GroupName                string                   `json:"group_name" bson:"group_name"`
	Retention                int64                    `json:"retention" bson:"retention"`
	RegistrationOwner        string                   `json:"registration_owner" bson:"registration_owner"`
	RegistrationVersion      int64                    `json:"registration_version" bson:"registration_version"`
	Version                  int64                    `json:"version" bson:"version"`
	MemberPublicationStarted bool                     `json:"member_publication_started" bson:"member_publication_started"`
	CallbackTemplate         []byte                   `json:"callback_template" bson:"callback_template"`
	CallbackPayload          []byte                   `json:"callback_payload,omitempty" bson:"callback_payload,omitempty"`
	Members                  []ChordMemberPublication `json:"members" bson:"members"`
	CallbackState            ChordCallbackState       `json:"callback_state" bson:"callback_state"`
	CallbackVersion          int64                    `json:"callback_version" bson:"callback_version"`
	CallbackLeaseOwner       string                   `json:"callback_lease_owner,omitempty" bson:"callback_lease_owner,omitempty"`
	CallbackLeaseExpiresAt   time.Time                `json:"callback_lease_expires_at,omitempty" bson:"callback_lease_expires_at,omitempty"`
	CallbackAttempts         int                      `json:"callback_attempts" bson:"callback_attempts"`
	CallbackNextAttemptAt    time.Time                `json:"callback_next_attempt_at,omitempty" bson:"callback_next_attempt_at,omitempty"`
	CallbackConfirmationAt   time.Time                `json:"callback_confirmation_at,omitempty" bson:"callback_confirmation_at,omitempty"`
	CallbackLastError        string                   `json:"callback_last_error,omitempty" bson:"callback_last_error,omitempty"`
	TerminalOutcome          CallbackTerminalOutcome  `json:"terminal_outcome,omitempty" bson:"terminal_outcome,omitempty"`
	TerminalAt               time.Time                `json:"terminal_at,omitempty" bson:"terminal_at,omitempty"`
	TerminalExpireAt         *time.Time               `json:"terminal_expire_at,omitempty" bson:"terminal_expire_at,omitempty"`
	CreatedAt                time.Time                `json:"created_at" bson:"created_at"`
	UpdatedAt                time.Time                `json:"updated_at" bson:"updated_at"`
}

type ChordMemberClaim struct {
	DeliveryKey   string
	Ordinal       int
	Owner         string
	Now           time.Time
	LeaseDuration time.Duration
}

type ChordMemberLease struct {
	DeliveryKey string
	Ordinal     int
	TaskID      string
	Payload     []byte
	Owner       string
	Version     int64
	ExpiresAt   time.Time
}

type ChordCallbackClaim struct {
	DeliveryKey   string
	Owner         string
	Now           time.Time
	LeaseDuration time.Duration
}

type ChordCallbackLease struct {
	DeliveryKey string
	Payload     []byte
	Owner       string
	Version     int64
	ExpiresAt   time.Time
}

type ChordPublishOutcome struct {
	Kind                 ChordPublishOutcomeKind
	Now                  time.Time
	ConfirmationDeadline time.Time
	NextAttemptAt        time.Time
	Error                string
}

type ChordScan struct {
	Cursor string
	Limit  int
	Now    time.Time
}

type ChordDeliveryPage struct {
	Deliveries []ChordDelivery
	NextCursor string
}

type DurableChordBackend interface {
	RegisterChord(context.Context, ChordRegistration) (ChordRegistrationRef, error)
	AbortRegistration(context.Context, ChordRegistrationRef) error
	ClaimMemberPublication(context.Context, ChordMemberClaim) (ChordMemberLease, bool, error)
	RecordMemberPublishOutcome(context.Context, ChordMemberLease, ChordPublishOutcome) error
	RecordMemberTerminal(context.Context, string, int, string, MemberTerminalOutcome, []*task.Result) error
	ScanChordDeliveries(context.Context, ChordScan) (ChordDeliveryPage, error)
	ReconcileChord(context.Context, string) error
	ClaimCallbackPublication(context.Context, ChordCallbackClaim) (ChordCallbackLease, bool, error)
	RecordCallbackPublishOutcome(context.Context, ChordCallbackLease, ChordPublishOutcome) error
	RecordCallbackTerminal(context.Context, string, CallbackTerminalOutcome) error
	CleanupTerminalChordDeliveries(context.Context, time.Time, int) (int, error)
}

func NormalizeChordRetention(value int64) int64 {
	if value == 0 {
		return DefaultChordRetentionSeconds
	}
	return value
}

func ChordDeliveryKey(groupID, callbackID string) string {
	groupSum := sha256.Sum256([]byte(groupID))
	deliverySum := sha256.Sum256([]byte("gkit-chord-v1\x00" + groupID + "\x00" + callbackID))
	return "chord:v1:" + hex.EncodeToString(groupSum[:]) + ":" + hex.EncodeToString(deliverySum[:])
}

func NewChordOwner() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate chord owner: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func FinalizeChordRegistration(reg *ChordRegistration) error {
	if reg == nil {
		return fmt.Errorf("%w: nil registration", ErrChordInvalidInput)
	}
	reg.Retention = NormalizeChordRetention(reg.Retention)
	if reg.DeliveryKey == "" {
		var callback task.Signature
		if err := json.Unmarshal(reg.Callback, &callback); err != nil {
			return fmt.Errorf("decode callback registration: %w", err)
		}
		reg.DeliveryKey = ChordDeliveryKey(reg.GroupID, callback.ID)
	}
	definition := *reg
	definition.DefinitionHash = ""
	body, err := json.Marshal(definition)
	if err != nil {
		return fmt.Errorf("marshal chord definition: %w", err)
	}
	sum := sha256.Sum256(body)
	reg.DefinitionHash = hex.EncodeToString(sum[:])
	return nil
}

func NewChordDelivery(reg ChordRegistration, owner string, now time.Time) ChordDelivery {
	members := make([]ChordMemberPublication, len(reg.Members))
	for index, member := range reg.Members {
		members[index] = ChordMemberPublication{
			Ordinal: member.Ordinal,
			TaskID:  member.TaskID,
			Payload: append([]byte(nil), member.Payload...),
			State:   ChordMemberSetup,
			Version: 1,
		}
	}
	return ChordDelivery{
		DeliveryKey:         reg.DeliveryKey,
		DefinitionHash:      reg.DefinitionHash,
		GroupID:             reg.GroupID,
		GroupName:           reg.GroupName,
		Retention:           NormalizeChordRetention(reg.Retention),
		RegistrationOwner:   owner,
		RegistrationVersion: 1,
		Version:             1,
		CallbackTemplate:    append([]byte(nil), reg.Callback...),
		Members:             members,
		CallbackState:       ChordWaiting,
		CallbackVersion:     1,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

func ChordRegistrationMatches(delivery *ChordDelivery, reg ChordRegistration) bool {
	return delivery != nil && delivery.GroupID == reg.GroupID && delivery.DefinitionHash == reg.DefinitionHash
}

func PrepareChordMembers(delivery *ChordDelivery, now time.Time) {
	for index := range delivery.Members {
		if delivery.Members[index].State == ChordMemberSetup {
			delivery.Members[index].State = ChordMemberReady
			delivery.Members[index].Version++
		}
	}
	delivery.Version++
	delivery.UpdatedAt = now
}

func ClaimChordMember(delivery *ChordDelivery, claim ChordMemberClaim) (ChordMemberLease, bool, error) {
	if delivery == nil || claim.Ordinal < 0 || claim.Ordinal >= len(delivery.Members) {
		return ChordMemberLease{}, false, ErrChordNotFound
	}
	now := claim.Now
	if now.IsZero() {
		now = time.Now()
	}
	member := &delivery.Members[claim.Ordinal]
	switch member.State {
	case ChordMemberLeased:
		if member.LeaseExpiresAt.After(now) {
			return ChordMemberLease{}, false, nil
		}
		member.State = ChordMemberReady
	case ChordMemberPublished, ChordMemberPublishUnknown:
		if member.ConfirmationDeadline.After(now) {
			return ChordMemberLease{}, false, nil
		}
		member.State = ChordMemberReady
	case ChordMemberReady:
	case ChordMemberSetup, ChordMemberTerminal:
		return ChordMemberLease{}, false, nil
	default:
		return ChordMemberLease{}, false, fmt.Errorf("unknown member state %q", member.State)
	}
	if member.NextAttemptAt.After(now) {
		return ChordMemberLease{}, false, nil
	}
	duration := claim.LeaseDuration
	if duration <= 0 {
		duration = 30 * time.Second
	}
	member.State = ChordMemberLeased
	member.LeaseOwner = claim.Owner
	member.LeaseExpiresAt = now.Add(duration)
	member.Version++
	member.Attempts++
	delivery.MemberPublicationStarted = true
	delivery.Version++
	delivery.UpdatedAt = now
	return ChordMemberLease{
		DeliveryKey: delivery.DeliveryKey,
		Ordinal:     member.Ordinal,
		TaskID:      member.TaskID,
		Payload:     append([]byte(nil), member.Payload...),
		Owner:       claim.Owner,
		Version:     member.Version,
		ExpiresAt:   member.LeaseExpiresAt,
	}, true, nil
}

func ApplyChordMemberPublishOutcome(delivery *ChordDelivery, lease ChordMemberLease, outcome ChordPublishOutcome) error {
	if delivery == nil || lease.Ordinal < 0 || lease.Ordinal >= len(delivery.Members) {
		return ErrChordNotFound
	}
	member := &delivery.Members[lease.Ordinal]
	if member.State != ChordMemberLeased || member.LeaseOwner != lease.Owner || member.Version != lease.Version {
		return ErrChordLeaseLost
	}
	now := outcome.Now
	if now.IsZero() {
		now = time.Now()
	}
	member.LeaseOwner = ""
	member.LeaseExpiresAt = time.Time{}
	member.LastError = chordErrorSummary("member_publish", outcome)
	switch outcome.Kind {
	case ChordPublishOutcomeSucceeded:
		member.State = ChordMemberPublished
		member.ConfirmationDeadline = deadlineOrDefault(outcome.ConfirmationDeadline, now.Add(30*time.Second))
	case ChordPublishOutcomeUnknown:
		member.State = ChordMemberPublishUnknown
		member.ConfirmationDeadline = deadlineOrDefault(outcome.ConfirmationDeadline, now.Add(30*time.Second))
	case ChordPublishOutcomeRejected:
		member.State = ChordMemberTerminal
		member.Receipt = &ChordMemberReceipt{TaskID: member.TaskID, Outcome: MemberTerminalFailure}
		setChordSuppressed(delivery, now)
	default:
		return fmt.Errorf("unknown publish outcome %q", outcome.Kind)
	}
	if !outcome.NextAttemptAt.IsZero() {
		member.NextAttemptAt = outcome.NextAttemptAt
	}
	member.Version++
	delivery.Version++
	delivery.UpdatedAt = now
	return nil
}

func ApplyChordMemberTerminal(delivery *ChordDelivery, ordinal int, taskID string, outcome MemberTerminalOutcome, results []*task.Result, now time.Time) error {
	if delivery == nil || ordinal < 0 || ordinal >= len(delivery.Members) {
		return ErrChordNotFound
	}
	member := &delivery.Members[ordinal]
	if member.TaskID != taskID {
		return ErrChordReceiptConflict
	}
	copyResults, err := cloneChordResults(results)
	if err != nil {
		return err
	}
	if member.Receipt != nil {
		candidate := ChordMemberReceipt{TaskID: taskID, Outcome: outcome, Results: copyResults}
		existing, _ := json.Marshal(member.Receipt)
		wanted, _ := json.Marshal(candidate)
		if bytes.Equal(existing, wanted) {
			return nil
		}
		return ErrChordReceiptConflict
	}
	member.State = ChordMemberTerminal
	member.LeaseOwner = ""
	member.LeaseExpiresAt = time.Time{}
	member.Receipt = &ChordMemberReceipt{TaskID: taskID, Outcome: outcome, Results: copyResults}
	member.Version++
	delivery.Version++
	delivery.UpdatedAt = now
	if outcome == MemberTerminalFailure {
		setChordSuppressed(delivery, now)
		return nil
	}
	for index := range delivery.Members {
		if delivery.Members[index].Receipt == nil {
			return nil
		}
		if delivery.Members[index].Receipt.Outcome == MemberTerminalFailure {
			setChordSuppressed(delivery, now)
			return nil
		}
	}
	payload, err := buildChordCallbackPayload(delivery)
	if err != nil {
		return err
	}
	if len(delivery.CallbackPayload) == 0 {
		delivery.CallbackPayload = payload
	}
	delivery.CallbackState = ChordReady
	delivery.CallbackVersion++
	return nil
}

func ClaimChordCallback(delivery *ChordDelivery, claim ChordCallbackClaim) (ChordCallbackLease, bool, error) {
	if delivery == nil {
		return ChordCallbackLease{}, false, ErrChordNotFound
	}
	now := claim.Now
	if now.IsZero() {
		now = time.Now()
	}
	switch delivery.CallbackState {
	case ChordLeased:
		if delivery.CallbackLeaseExpiresAt.After(now) {
			return ChordCallbackLease{}, false, nil
		}
		delivery.CallbackState = ChordReady
	case ChordPublished, ChordPublishUnknown:
		if delivery.CallbackConfirmationAt.After(now) {
			return ChordCallbackLease{}, false, nil
		}
		delivery.CallbackState = ChordReady
	case ChordReady:
	case ChordWaiting, ChordDelivered, ChordSuppressed:
		return ChordCallbackLease{}, false, nil
	default:
		return ChordCallbackLease{}, false, fmt.Errorf("unknown callback state %q", delivery.CallbackState)
	}
	if delivery.CallbackNextAttemptAt.After(now) {
		return ChordCallbackLease{}, false, nil
	}
	duration := claim.LeaseDuration
	if duration <= 0 {
		duration = 30 * time.Second
	}
	delivery.CallbackState = ChordLeased
	delivery.CallbackLeaseOwner = claim.Owner
	delivery.CallbackLeaseExpiresAt = now.Add(duration)
	delivery.CallbackVersion++
	delivery.CallbackAttempts++
	delivery.Version++
	delivery.UpdatedAt = now
	return ChordCallbackLease{
		DeliveryKey: delivery.DeliveryKey,
		Payload:     append([]byte(nil), delivery.CallbackPayload...),
		Owner:       claim.Owner,
		Version:     delivery.CallbackVersion,
		ExpiresAt:   delivery.CallbackLeaseExpiresAt,
	}, true, nil
}

func ApplyChordCallbackPublishOutcome(delivery *ChordDelivery, lease ChordCallbackLease, outcome ChordPublishOutcome) error {
	if delivery == nil {
		return ErrChordNotFound
	}
	if delivery.CallbackState != ChordLeased || delivery.CallbackLeaseOwner != lease.Owner || delivery.CallbackVersion != lease.Version {
		return ErrChordLeaseLost
	}
	now := outcome.Now
	if now.IsZero() {
		now = time.Now()
	}
	delivery.CallbackLeaseOwner = ""
	delivery.CallbackLeaseExpiresAt = time.Time{}
	delivery.CallbackLastError = chordErrorSummary("callback_publish", outcome)
	switch outcome.Kind {
	case ChordPublishOutcomeSucceeded:
		delivery.CallbackState = ChordPublished
		delivery.CallbackConfirmationAt = deadlineOrDefault(outcome.ConfirmationDeadline, now.Add(30*time.Second))
	case ChordPublishOutcomeUnknown:
		delivery.CallbackState = ChordPublishUnknown
		delivery.CallbackConfirmationAt = deadlineOrDefault(outcome.ConfirmationDeadline, now.Add(30*time.Second))
	case ChordPublishOutcomeRejected:
		delivery.CallbackState = ChordReady
		delivery.CallbackNextAttemptAt = deadlineOrDefault(outcome.NextAttemptAt, now.Add(time.Second))
	default:
		return fmt.Errorf("unknown publish outcome %q", outcome.Kind)
	}
	delivery.CallbackVersion++
	delivery.Version++
	delivery.UpdatedAt = now
	return nil
}

func ApplyChordCallbackTerminal(delivery *ChordDelivery, outcome CallbackTerminalOutcome, now time.Time) error {
	if delivery == nil {
		return ErrChordNotFound
	}
	if delivery.CallbackState == ChordDelivered {
		if delivery.TerminalOutcome == outcome {
			return nil
		}
		return ErrChordTerminalConflict
	}
	if delivery.CallbackState == ChordSuppressed {
		return ErrChordTerminalConflict
	}
	delivery.CallbackState = ChordDelivered
	delivery.TerminalOutcome = outcome
	setChordTerminalTime(delivery, now)
	delivery.CallbackVersion++
	delivery.Version++
	delivery.UpdatedAt = now
	return nil
}

func SortChordDeliveries(deliveries []ChordDelivery) {
	sort.Slice(deliveries, func(i, j int) bool { return deliveries[i].DeliveryKey < deliveries[j].DeliveryKey })
}

func chordErrorSummary(operation string, outcome ChordPublishOutcome) string {
	if outcome.Error == "" {
		return ""
	}
	category := strings.ToLower(string(outcome.Kind))
	return "operation=" + operation + " category=" + category
}

func deadlineOrDefault(value, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value
}

func cloneChordResults(results []*task.Result) ([]*task.Result, error) {
	body, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("serialize chord member results: %w", err)
	}
	var copyResults []*task.Result
	if err := json.Unmarshal(body, &copyResults); err != nil {
		return nil, fmt.Errorf("deserialize chord member results: %w", err)
	}
	return copyResults, nil
}

func buildChordCallbackPayload(delivery *ChordDelivery) ([]byte, error) {
	var callback task.Signature
	if err := json.Unmarshal(delivery.CallbackTemplate, &callback); err != nil {
		return nil, fmt.Errorf("decode chord callback template: %w", err)
	}
	for ordinal := range delivery.Members {
		for _, result := range delivery.Members[ordinal].Receipt.Results {
			callback.Args = append(callback.Args, task.Arg{Type: result.Type, Value: result.Value})
		}
	}
	return json.Marshal(&callback)
}

func setChordSuppressed(delivery *ChordDelivery, now time.Time) {
	if delivery.CallbackState == ChordDelivered || delivery.CallbackState == ChordSuppressed {
		return
	}
	delivery.CallbackState = ChordSuppressed
	delivery.TerminalOutcome = CallbackTerminalFailure
	setChordTerminalTime(delivery, now)
	delivery.CallbackVersion++
}

func setChordTerminalTime(delivery *ChordDelivery, now time.Time) {
	if now.IsZero() {
		now = time.Now()
	}
	delivery.TerminalAt = now
	if delivery.Retention < 0 {
		delivery.TerminalExpireAt = nil
		return
	}
	expires := now.Add(time.Duration(NormalizeChordRetention(delivery.Retention)) * time.Second)
	delivery.TerminalExpireAt = &expires
}
