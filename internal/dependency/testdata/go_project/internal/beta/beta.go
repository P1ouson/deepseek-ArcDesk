package beta

import (
	"example.com/testproj/internal/gamma"
	"github.com/example/extpkg"
)

func B() string {
	gamma.C()
	extpkg.X()
	return "b"
}
