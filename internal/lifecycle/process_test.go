package lifecycle

import (
	"reflect"
	"testing"
)

func TestTaskkillArgsTargetsProcessTree(t *testing.T) {
	got := taskkillArgs(4321)
	want := []string{"/PID", "4321", "/T", "/F"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("taskkillArgs() = %q, want %q", got, want)
	}
}
