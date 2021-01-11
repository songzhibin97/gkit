package singleflight

import "golang.org/x/sync/singleflight"

type Group struct {
	singleflight.Group
}

// NewSingleFlight: 实例化
func NewSingleFlight() Singler {
	return &Group{}
}
