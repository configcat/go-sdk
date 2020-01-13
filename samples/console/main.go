package main

import (
	"fmt"
	"github.com/configcat/go-sdk"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	config := configcat.DefaultClientConfig()
	config.Logger = logger

	client := configcat.NewCustomClient("PKDVCLf-Hq-h-kCzMp-L7Q/HhOWfwVtZ0mb30i9wi17GQ", config)

	// create a user object to identify your user (optional)
	user := configcat.NewUser("##SOME-USER-IDENTIFICATION##")

	// get individual config values identified by a key and a user object
	value := client.GetValueForUser("isPOCFeatureEnabled", "", user)

	fmt.Println("isPOCFeatureEnabled: ", value)
}
