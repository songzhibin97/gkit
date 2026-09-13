package backend_mongodb

import (
	"github.com/songzhibin97/gkit/distributed/backend/chordtest"
	"testing"
)

func TestIssue167ResultPrecision(t *testing.T) { chordtest.RunResultPrecision(t, issue167Mongo(t)) }

func TestIssue167ResultConsumers(t *testing.T) { chordtest.RunResultConsumers(t, issue167Mongo(t)) }
