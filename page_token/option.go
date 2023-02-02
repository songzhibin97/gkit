package page_token

import (
	"time"

	"github.com/songzhibin97/gkit/encrypt/aes"

	"github.com/songzhibin97/gkit/options"
)

func SetMaxIndex(max int) options.Option {
	return func(o interface{}) {
		if t, ok := o.(*token); ok {
			t.maxIndex = max
		}
	}
}

func SetMaxElements(max int) options.Option {
	return func(o interface{}) {
		if t, ok := o.(*token); ok {
			t.maxElements = max
		}
	}
}

func SetSalt(salt string) options.Option {
	return func(o interface{}) {
		if t, ok := o.(*token); ok {
			t.salt = aes.PadKey(salt)
		}
	}
}

func SetTimeLimitation(d time.Duration) options.Option {
	return func(o interface{}) {
		if t, ok := o.(*token); ok {
			t.timeLimitation = d
		}
	}
}
