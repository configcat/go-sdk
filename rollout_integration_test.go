package configcat

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRolloutIntegration(t *testing.T) {
	config := DefaultClientConfig()
	config.PolicyFactory = func(configProvider ConfigProvider, store *ConfigStore) RefreshPolicy {
		return NewExpiringCachePolicy(configProvider, store, time.Second * 120, false)
	}
	client := NewCustomClient("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A",
		config)

	defer client.Close()

	file, fileErr := os.Open("resources/testmatrix.csv")
	if fileErr != nil {
		log.Fatal(fileErr)
	}

	reader := csv.NewReader(bufio.NewReader(file))
	reader.Comma = ';'

	header, _ := reader.Read()
	settingKeys := header[4:]

	var errors []string

	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}

		var user *User
		if len(line[0]) > 0 && line[0] != "##nouserobject##" {
			custom := map[string]string{}
			if len(line[3]) > 0 {
				custom["Custom1"] = line[3]
			}

			user = NewUserWithAdditionalAttributes(line[0], line[1], line[2], custom)
		}

		var i = 0
		for _, settingKey := range settingKeys {
			val := client.GetValueForUser(settingKey, nil, user)
			expected := line[i + 4]
			boolVal, ok := val.(bool)
			if ok {
				expectedVal, err := strconv.ParseBool(strings.ToLower(expected))
				if err == nil && boolVal != expectedVal {
					err := fmt.Sprintf("Identifier: %s, Key: %s. Expected: %v, Result: %v \n", line[0], settingKey, expectedVal, boolVal)
					errors = append(errors, err)
					fmt.Print(err)
				}
				i++
				continue
			}


			intVal, ok := val.(int)
			if ok {
				expectedVal, err := strconv.Atoi(strings.ToLower(expected))
				if err == nil && intVal != expectedVal {
					err := fmt.Sprintf("Identifier: %s, Key: %s. Expected: %v, Result: %v \n", line[0], settingKey, expectedVal, intVal)
					errors = append(errors, err)
					fmt.Print(err)
				}
				i++
				continue
			}

			doubleVal, ok := val.(float64)
			if ok {
				expectedVal, err := strconv.ParseFloat(strings.ToLower(expected), 64)
				if err == nil && doubleVal != expectedVal {
					err := fmt.Sprintf("Identifier: %s, Key: %s. Expected: %v, Result: %v \n", line[0], settingKey, expectedVal, doubleVal)
					errors = append(errors, err)
					fmt.Print(err)
				}
				i++
				continue
			}

			stringVal, ok := val.(string)
			if ok {
				expectedVal := strings.ToLower(expected)
				if strings.ToLower(stringVal) != expectedVal {
					err := fmt.Sprintf("Identifier: %s, Key: %s. Expected: %v, Result: %v \n", line[0], settingKey, expectedVal, strings.ToLower(stringVal))
					errors = append(errors, err)
					fmt.Print(err)
				}
				i++
				continue
			}

			t.Fatalf("Value was not handled %v", val)
		}
	}

	if len(errors) > 0 {
		t.Error("Expecting no errors")
	}
}
