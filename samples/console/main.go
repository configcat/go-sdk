package main

import (
	"fmt"
	"gopkg.in/configcat/go-sdk.v1"
)

func main() {
	client := configcat.NewClient("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A")

	// create a user object to identify the caller
	user := configcat.NewUser("key")

	// get individual config values identified by a key for a user
	value := client.GetValueForUser("keySampleText", "", user)

	fmt.Println("keySampleText: ", value)
}