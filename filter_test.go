package main

import (
	"net/http"
	"reflect"
	"testing"
)

func TestFilter_filter(t *testing.T) {
	type fields struct {
		Enabled  bool
		Locator  string
		Callsign string
		Bands    []string
		Modes    []string
	}
	type args struct {
		spot Payload
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &Filter{
				Enabled:  tt.fields.Enabled,
				Locator:  tt.fields.Locator,
				Callsign: tt.fields.Callsign,
				Bands:    tt.fields.Bands,
				Modes:    tt.fields.Modes,
			}
			if got := filter.filter(tt.args.spot); got != tt.want {
				t.Errorf("filter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewFilter(t *testing.T) {
	type args struct {
		config  Config
		request *http.Request
	}
	tests := []struct {
		name string
		args args
		want Filter
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewFilter(tt.args.config, tt.args.request); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewFilter() = %v, want %v", got, tt.want)
			}
		})
	}
}
