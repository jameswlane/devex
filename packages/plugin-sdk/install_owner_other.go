//go:build !unix

package sdk

import (
	"fmt"
	"os"
)

func checkDirectoryOwner(_ os.FileInfo, _ int) error {
	return fmt.Errorf("Unix plugin ownership is unsupported on this platform")
}
