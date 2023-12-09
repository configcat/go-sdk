package main

import (
	"fmt"
	"github.com/configcat/go-sdk/v9"
)

func main() {
	client := configcat.NewCustomClient(configcat.Config{
		SDKKey:   "PKDVCLf-Hq-h-kCzMp-L7Q/HhOWfwVtZ0mb30i9wi17GQ",
		LogLevel: configcat.LogLevelInfo,
	})

	// create a user object to identify your user (optional)
	user := &configcat.UserData{
		Identifier: "##SOME-USER-IDENTIFICATION##",
		Email:      "configcat@example.com",
	}

	// get individual config values identified by a key and a user object
	value := client.GetBoolValue("isPOCFeatureEnabled", false, user)

	fmt.Println("isPOCFeatureEnabled: ", value)
}
