package ebml

import (
	"os"
	"testing"
)

func TestSeek(t *testing.T) {
	f, _ := os.Open("lrs_test.go")
	lrs := newLimitedReadSeeker(f, 1000)
	t.Log(lrs)
	curr, _ := lrs.Seek(0, 1)
	if curr != 0 {
		t.Fatal("1")
	}
	lrs.Seek(100, 1)
	curr2, _ := lrs.Seek(100, 0)
	if curr2 != 100 {
		t.Fatal("2")
	}
	if lrs.LimitedReader.N != 900 {
		t.Fatal("3")
	}
	t.Log(lrs)
	lrs2 := newLimitedReadSeeker(lrs, 500)
	curr2, _ = lrs2.Seek(0, 1)
	if curr2 != 100 {
		t.Fatal("4")
	}
	t.Log(curr2, lrs2)
	curr3, _ := lrs2.Seek(200, 1)
	if curr3 != 300 {
		t.Fatal("5")
	}
	curr4, _ := lrs.Seek(0, 1)
	if curr4 != 300 {
		t.Fatal("6")
	}
	t.Log(lrs2)

}
