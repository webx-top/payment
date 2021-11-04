package xunhupay

import (
	"strings"
	"testing"
)

func TestTrim(t *testing.T) {
	if strings.Trim(` ;;a;; `, ` ;`) != `a` {
		panic(``)
	}
}
