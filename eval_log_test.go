package configcat

import (
	"context"
	"encoding/json"
	qt "github.com/frankban/quicktest"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var testKeys = []string{
	"1_targeting_rule",
	"2_targeting_rules",
	"and_rules",
	"comparators",
	"epoch_date_validation",
	"list_truncation",
	"number_validation",
	"options_after_targeting_rule",
	"options_based_on_custom_attr",
	"options_based_on_user_id",
	"options_within_targeting_rule",
	"prerequisite_flag",
	"segment",
	"semver_validation",
	"simple_value",
}

type testSuite struct {
	ConfigUrl    string      `json:"configUrl"`
	SdkKey       string      `json:"sdkKey"`
	JsonOverride string      `json:"jsonOverride"`
	Tests        []*testCase `json:"tests"`
}

type testCase struct {
	Key          string                 `json:"key"`
	DefaultValue interface{}            `json:"defaultValue"`
	ReturnValue  interface{}            `json:"returnValue"`
	ExpectedLog  string                 `json:"expectedLog"`
	User         map[string]interface{} `json:"user"`
}

func TestEvalLogs(t *testing.T) {
	for _, key := range testKeys {
		runEvalTest(t, key)
	}
}

func runEvalTest(t *testing.T, key string) {
	data, err := os.ReadFile(filepath.Join("resources", "evaluationlog", key+".json"))
	if err != nil {
		log.Fatal(err)
	}
	var suite testSuite
	err = json.Unmarshal(data, &suite)
	if err != nil {
		log.Fatal(err)
	}

	logger := newTestLogger(t).(*testLogger)
	var config Config
	if suite.JsonOverride != "" {
		config = Config{
			FlagOverrides: &FlagOverrides{
				FilePath: filepath.Join("resources", "evaluationlog", "_overrides", suite.JsonOverride),
				Behavior: LocalOnly,
			},
			PollingMode: Manual,
			Logger:      logger,
			LogLevel:    LogLevelInfo,
		}
	} else {
		config = Config{
			SDKKey:      suite.SdkKey,
			PollingMode: Manual,
			Logger:      logger,
			LogLevel:    LogLevelInfo,
		}
	}

	client := NewCustomClient(config)
	defer client.Close()
	_ = client.Refresh(context.Background())

	t.Run(key, func(t *testing.T) {
		for _, test := range suite.Tests {
			t.Run(test.ExpectedLog, func(t *testing.T) {
				c := qt.New(t)
				logger.Clear()
				exp, err := os.ReadFile(filepath.Join("resources", "evaluationlog", key, test.ExpectedLog))
				if err != nil {
					log.Fatal(err)
				}
				val := client.Snapshot(test.User).GetValue(test.Key)
				switch v := val.(type) {
				case int:
					c.Assert(float64(v), qt.Equals, test.ReturnValue)
				default:
					c.Assert(val, qt.Equals, test.ReturnValue)
				}

				logs := strings.Join(logger.Logs(), "\n")

				c.Assert(logs, qt.Equals, string(exp))
			})
		}
	})
}
