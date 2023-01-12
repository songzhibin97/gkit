package schedule

import (
	"github.com/songzhibin97/gkit/options"
)

type Interval int

const (
	Second         Interval = 1 << iota // Seconds field, default 0
	SecondOptional                      // Optional seconds field, default 0
	Minute                              // Minutes field, default 0
	Hour                                // Hours field, default 0
	Dom                                 // Day of month field, default *
	Month                               // Month field, default *
	Dow                                 // Day of week field, default *
	DowOptional                         // Optional day of week field, default *
	Descriptor                          // Allow descriptors such as @monthly, @weekly, etc.
)

var places = []Interval{
	Second,
	Minute,
	Hour,
	Dom,
	Month,
	Dow,
}

var defaults = []string{
	"0",
	"0",
	"0",
	"*",
	"*",
	"*",
}

type Config struct {
	interval Interval
}

func WithInterval(options Interval) options.Option {
	return func(o interface{}) {
		optionals := 0
		if options&DowOptional > 0 {
			optionals++
		}
		if options&SecondOptional > 0 {
			optionals++
		}
		if optionals > 1 {
			panic("multiple optionals may not be configured")
		}
		o.(*Config).interval = options
	}
}
