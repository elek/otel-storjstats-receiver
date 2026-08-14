package storjstatsreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseStatKey(t *testing.T) {
	tests := []struct {
		in       string
		wantName string
		wantTags map[string]string
		wantFld  string
	}{
		{
			in:       "pieces_writer,size=2m ravg",
			wantName: "pieces_writer",
			wantTags: map[string]string{"size": "2m"},
			wantFld:  "ravg",
		},
		{
			in:       "upload_success_size_bytes count",
			wantName: "upload_success_size_bytes",
			wantTags: map[string]string{},
			wantFld:  "count",
		},
		{
			in:       "hashstore,store=blob,op=write count",
			wantName: "hashstore",
			wantTags: map[string]string{"store": "blob", "op": "write"},
			wantFld:  "count",
		},
		{
			in:       "version_info",
			wantName: "version_info",
			wantTags: map[string]string{},
			wantFld:  "",
		},
		{
			in:       "queue,name=x length",
			wantName: "queue",
			wantTags: map[string]string{"name": "x"},
			wantFld:  "length",
		},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got := parseStatKey([]byte(tc.in))
			assert.Equal(t, tc.wantName, got.name)
			assert.Equal(t, tc.wantTags, got.tags)
			assert.Equal(t, tc.wantFld, got.field)
		})
	}
}

func TestParseStatKey_SkipsMalformedTagPair(t *testing.T) {
	// "bogus" (no '=') is dropped silently; well-formed pairs survive.
	got := parseStatKey([]byte("thing,bogus,ok=1 value"))
	assert.Equal(t, "thing", got.name)
	assert.Equal(t, map[string]string{"ok": "1"}, got.tags)
	assert.Equal(t, "value", got.field)
}
