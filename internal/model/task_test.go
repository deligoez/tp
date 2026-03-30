package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidTransition(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{"open to wip", StatusOpen, StatusWIP, true},
		{"wip to done", StatusWIP, StatusDone, true},
		{"done to open", StatusDone, StatusOpen, true},
		{"open to done invalid", StatusOpen, StatusDone, false},
		{"wip to open invalid", StatusWIP, StatusOpen, false},
		{"done to wip invalid", StatusDone, StatusWIP, false},
		{"open to open same", StatusOpen, StatusOpen, false},
		{"wip to wip same", StatusWIP, StatusWIP, false},
		{"done to done same", StatusDone, StatusDone, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ValidTransition(tt.from, tt.to))
		})
	}
}

func TestValidStatus(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"open is valid", "open", true},
		{"wip is valid", "wip", true},
		{"done is valid", "done", true},
		{"blocked is invalid", "blocked", false},
		{"deferred is invalid", "deferred", false},
		{"empty is invalid", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ValidStatus(tt.status))
		})
	}
}
