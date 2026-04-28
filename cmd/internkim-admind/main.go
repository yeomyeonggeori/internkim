package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/anthropic-lab/internkim/internal/admind"
)

func main() {
	configuration := admind.DefaultConfiguration()
	flag.StringVar(&configuration.ListenAddress, "listen", configuration.ListenAddress, "HTTP listen address")
	flag.StringVar(&configuration.MattermostBaseURL, "mattermost-url", configuration.MattermostBaseURL, "Mattermost upstream URL")
	flag.StringVar(&configuration.AdminPageBaseURL, "admin-page-url", configuration.AdminPageBaseURL, "InternKim admin page URL")
	flag.StringVar(&configuration.StateDirectory, "state-dir", configuration.StateDirectory, "admin job state directory")
	flag.StringVar(&configuration.AdminEmailPath, "admin-email-path", configuration.AdminEmailPath, "initial admin email file")
	flag.StringVar(&configuration.DeviceIDPath, "device-id-path", configuration.DeviceIDPath, "device ID file")
	flag.Parse()

	if errorValue := admind.Run(configuration); errorValue != nil {
		fmt.Fprintln(os.Stderr, errorValue)
		os.Exit(1)
	}
}
