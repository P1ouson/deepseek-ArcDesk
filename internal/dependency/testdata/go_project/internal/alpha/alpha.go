package alpha

import (
	"fmt"

	"example.com/testproj/internal/beta"
)

func A() {
	fmt.Println(beta.B())
}
