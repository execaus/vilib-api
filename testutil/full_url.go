package testutil

import "fmt"

func FullURI(version, target string) string {
	return fmt.Sprintf("/api/%s/%s", version, target)
}
