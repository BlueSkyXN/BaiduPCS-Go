package pcsutil_test

import (
	"fmt"
	"github.com/qjfoidnh/BaiduPCS-Go/pcsutil"
	"testing"
)

func TestWalkDir(t *testing.T) {
	files, err := pcsutil.WalkDir(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		fmt.Println(file)
	}
}
