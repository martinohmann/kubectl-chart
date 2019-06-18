package cmdutil

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// CheckErr prints err and exits with a non-zero code if err is not nil.
func CheckErr(err error) {
	if err == nil {
		return
	}

	msg := err.Error()

	if len(msg) > 0 {
		// add newline if needed
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}
		fmt.Fprint(os.Stderr, msg)
	}

	os.Exit(1)
}

// GetString retrieves a string flag from cmd.
func GetString(cmd *cobra.Command, flag string) string {
	v, err := cmd.Flags().GetString(flag)
	if err != nil {
		log.Fatal(err)
	}

	return v
}
