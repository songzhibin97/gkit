package backend_db

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	stdlog "log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/task"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const issue104LoggingSecret = "synthetic-secret-token-issue104"

func TestDurableChordSQLFailureDoesNotLogPayloads(t *testing.T) {
	var captured bytes.Buffer
	logConfig := gormlogger.Config{LogLevel: gormlogger.Info, Colorful: false, ParameterizedQueries: false}
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:issue104-log-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{
		Logger: gormlogger.New(stdlog.New(&captured, "", 0), logConfig),
	})
	if err != nil {
		t.Fatal(err)
	}
	b := &BackendSQLDB{gClient: db, resultExpire: -1}
	if err := b.autoMigrate(); err != nil {
		t.Fatal(err)
	}
	assertDurableSQLFailureDoesNotLogPayloads(t, b, &captured, "sqlite")
}

func TestDurableChordSQLFailureDoesNotLogPayloadsLive(t *testing.T) {
	tests := []struct {
		name   string
		driver string
		dbType string
		env    string
	}{
		{name: "mysql", driver: "mysql", dbType: "mysql", env: "GKIT_MYSQL_DSN"},
		{name: "postgres", driver: "pgx", dbType: "pgsql", env: "GKIT_POSTGRES_DSN"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dsn := os.Getenv(tc.env)
			if dsn == "" {
				t.Skip(tc.env + " is required for live durable logging test")
			}
			sqlDB, err := sql.Open(tc.driver, dsn)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = sqlDB.Close() })
			var captured bytes.Buffer
			logConfig := gormlogger.Config{LogLevel: gormlogger.Info, Colorful: false, ParameterizedQueries: false}
			value, err := NewBackendSQLDBE(sqlDB, -1, tc.dbType, &gorm.Config{
				Logger: gormlogger.New(stdlog.New(&captured, "", 0), logConfig),
			})
			if err != nil {
				t.Fatal(err)
			}
			assertDurableSQLFailureDoesNotLogPayloads(t, value.(*BackendSQLDB), &captured, tc.name)
		})
	}
}

func assertDurableSQLFailureDoesNotLogPayloads(t *testing.T, b *BackendSQLDB, captured *bytes.Buffer, dialect string) {
	t.Helper()
	registration, artifacts := issue104SensitiveRegistration(t, fmt.Sprintf("logging-%s-%d", dialect, time.Now().UnixNano()))
	ref, err := b.RegisterChord(context.Background(), registration)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.ReconcileChord(context.Background(), ref.DeliveryKey); err != nil {
		t.Fatal(err)
	}
	lease, claimed, err := b.ClaimMemberPublication(context.Background(), backend.ChordMemberClaim{
		DeliveryKey: ref.DeliveryKey,
		Ordinal:     0,
		Owner:       "logging-owner",
		Now:         time.Now(),
	})
	if err != nil || !claimed {
		t.Fatalf("member claim = %t, %v", claimed, err)
	}
	removeTrigger := installIssue104FailingUpdateTrigger(t, b, dialect)
	t.Cleanup(removeTrigger)
	captured.Reset()
	forcedErr := b.RecordMemberPublishOutcome(context.Background(), lease, backend.ChordPublishOutcome{
		Kind:  backend.ChordPublishOutcomeUnknown,
		Now:   time.Now(),
		Error: issue104LoggingSecret + ":" + artifacts.rawMember,
	})
	if forcedErr == nil || !strings.Contains(forcedErr.Error(), "forced durable update failure") {
		t.Fatalf("RecordMemberPublishOutcome error = %v, want observable forced failure", forcedErr)
	}
	logs := captured.String()
	for _, forbidden := range []string{
		issue104LoggingSecret,
		artifacts.rawMember,
		artifacts.rawCallback,
		artifacts.memberBase64,
		artifacts.callbackBase64,
		"private-arg-value",
		"private-meta-value",
	} {
		if strings.Contains(logs, forbidden) {
			t.Fatalf("captured GORM logger leaked %q in %q", forbidden, logs)
		}
	}
	if !strings.Contains(forcedErr.Error(), "update sql chord delivery") {
		t.Fatalf("RecordMemberPublishOutcome error lacks operation context: %v", forcedErr)
	}
}

func installIssue104FailingUpdateTrigger(t *testing.T, b *BackendSQLDB, dialect string) func() {
	t.Helper()
	const trigger = "gkit_issue104_fail_chord_update"
	switch dialect {
	case "sqlite":
		if err := b.gClient.Exec("DROP TRIGGER IF EXISTS " + trigger).Error; err != nil {
			t.Fatal(err)
		}
		if err := b.gClient.Exec("CREATE TRIGGER " + trigger + " BEFORE UPDATE ON chord_deliveries BEGIN SELECT RAISE(FAIL, 'forced durable update failure'); END").Error; err != nil {
			t.Fatal(err)
		}
		return func() { _ = b.gClient.Exec("DROP TRIGGER IF EXISTS " + trigger).Error }
	case "mysql":
		if err := b.gClient.Exec("DROP TRIGGER IF EXISTS " + trigger).Error; err != nil {
			t.Fatal(err)
		}
		if err := b.gClient.Exec("CREATE TRIGGER " + trigger + " BEFORE UPDATE ON chord_deliveries FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'forced durable update failure'").Error; err != nil {
			t.Fatal(err)
		}
		return func() { _ = b.gClient.Exec("DROP TRIGGER IF EXISTS " + trigger).Error }
	case "postgres":
		if err := b.gClient.Exec("DROP TRIGGER IF EXISTS " + trigger + " ON chord_deliveries").Error; err != nil {
			t.Fatal(err)
		}
		if err := b.gClient.Exec("CREATE OR REPLACE FUNCTION gkit_issue104_fail_chord_update_fn() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced durable update failure'; END $$").Error; err != nil {
			t.Fatal(err)
		}
		if err := b.gClient.Exec("CREATE TRIGGER " + trigger + " BEFORE UPDATE ON chord_deliveries FOR EACH ROW EXECUTE FUNCTION gkit_issue104_fail_chord_update_fn()").Error; err != nil {
			t.Fatal(err)
		}
		return func() {
			_ = b.gClient.Exec("DROP TRIGGER IF EXISTS " + trigger + " ON chord_deliveries").Error
			_ = b.gClient.Exec("DROP FUNCTION IF EXISTS gkit_issue104_fail_chord_update_fn()").Error
		}
	default:
		t.Fatalf("unsupported trigger dialect %q", dialect)
		return func() {}
	}
}

type issue104SensitiveArtifacts struct {
	rawMember      string
	rawCallback    string
	memberBase64   string
	callbackBase64 string
}

func issue104SensitiveRegistration(t *testing.T, groupID string) (backend.ChordRegistration, issue104SensitiveArtifacts) {
	t.Helper()
	deliveryKey := backend.ChordDeliveryKey(groupID, "logging-callback")
	callback := task.NewSignature("logging-callback", "callback")
	callback.Args = []task.Arg{{Type: "string", Value: "private-arg-value"}}
	callback.Meta.Set("private-meta", "private-meta-value")
	callback.Meta.Set(backend.DurableChordDeliveryKeyMeta, deliveryKey)
	callbackPayload, err := json.Marshal(callback)
	if err != nil {
		t.Fatal(err)
	}
	member := task.NewSignature(groupID+"-member", "member")
	member.GroupID = groupID
	member.Args = []task.Arg{{Type: "string", Value: "private-arg-value"}}
	member.Meta.Set("private-meta", "private-meta-value")
	member.Meta.Set(backend.DurableChordDeliveryKeyMeta, deliveryKey)
	member.Meta.Set(backend.DurableChordMemberMeta, true)
	member.Meta.Set(backend.DurableChordMemberOrdinal, 0)
	memberPayload, err := json.Marshal(member)
	if err != nil {
		t.Fatal(err)
	}
	registration := backend.ChordRegistration{
		GroupID:   groupID,
		GroupName: "logging-group",
		Retention: -1,
		Callback:  callbackPayload,
		Members:   []backend.ChordMemberRegistration{{Ordinal: 0, TaskID: member.ID, Payload: memberPayload}},
	}
	if err := backend.FinalizeChordRegistration(&registration); err != nil {
		t.Fatal(err)
	}
	return registration, issue104SensitiveArtifacts{
		rawMember:      string(memberPayload),
		rawCallback:    string(callbackPayload),
		memberBase64:   base64.StdEncoding.EncodeToString(memberPayload),
		callbackBase64: base64.StdEncoding.EncodeToString(callbackPayload),
	}
}
