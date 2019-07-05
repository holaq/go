// Windows environment variables.

package signal

import "fmt"

func AddKillListener(callbacks ...func()) {
	fmt.Println("Warning! cannot call 'AddKillListener' at windows's platform")
}
