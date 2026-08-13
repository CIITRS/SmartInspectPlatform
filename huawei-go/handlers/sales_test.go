package handlers

import "testing"

func TestValidPackageConfiguration(t *testing.T) {
	for _, test := range []struct {
		count, interval int
		want            bool
	}{{4, 90, true}, {1, 1, true}, {0, 90, false}, {4, 0, false}, {-1, 90, false}} {
		if got := validPackageConfiguration(test.count, test.interval); got != test.want {
			t.Fatalf("validPackageConfiguration(%d, %d) = %v, want %v", test.count, test.interval, got, test.want)
		}
	}
}
