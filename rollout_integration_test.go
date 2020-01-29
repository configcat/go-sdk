package configcat

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestRolloutIntegration(t *testing.T) {
	doIntegrationTest("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A", "testmatrix.csv", AutoPoll(120), t)
	doIntegrationTest("PKDVCLf-Hq-h-kCzMp-L7Q/BAr3KgLTP0ObzKnBTo5nhA", "testmatrix_semantic.csv", LazyLoad(120, false), t)
	doIntegrationTest("PKDVCLf-Hq-h-kCzMp-L7Q/uGyK3q9_ckmdxRyI7vjwCw", "testmatrix_number.csv", ManualPoll(), t)
	doIntegrationTest("PKDVCLf-Hq-h-kCzMp-L7Q/q6jMCFIp-EmuAfnmZhPY7w", "testmatrix_semantic_2.csv", AutoPoll(120), t)
}

func doIntegrationTest(apiKey string, fileName string, mode RefreshMode, t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	client := NewCustomClient(apiKey, ClientConfig{ Logger: logger, Mode: mode })
	client.Refresh()
	defer client.Close()

	file, fileErr := os.Open("resources/" + fileName)
	if fileErr != nil {
		log.Fatal(fileErr)
	}

	reader := csv.NewReader(bufio.NewReader(file))
	reader.Comma = ';'

	header, _ := reader.Read()
	settingKeys := header[4:]
	customKey := header[3]

	var errors []string

	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}

		var user *User
		if len(line[0]) > 0 && line[0] != "##null##" {

			email := ""
			country := ""
			identifier := line[0]

			if len(line[1]) > 0 && line[1] != "##null##" {
				email = line[1]
			}

			if len(line[2]) > 0 && line[2] != "##null##" {
				country = line[2]
			}

			custom := map[string]string{}
			if len(line[3]) > 0 && line[3] != "##null##" {
				custom[customKey] = line[3]
			}

			user = NewUserWithAdditionalAttributes(identifier, email, country, custom)
		}

		var i = 0
		for _, settingKey := range settingKeys {
			val := client.GetValueForUser(settingKey, nil, user)
			expected := line[i+4]
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
