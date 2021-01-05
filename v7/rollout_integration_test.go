package configcat

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const (
	valueKind     = 0
	variationKind = 1
)

type integrationTest struct {
	sdkKey   string
	fileName string
	kind     int
}

var integrationTests = []integrationTest{{
	sdkKey:   "PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A",
	fileName: "testmatrix.csv",
	kind:     valueKind,
}, {
	sdkKey:   "PKDVCLf-Hq-h-kCzMp-L7Q/BAr3KgLTP0ObzKnBTo5nhA",
	fileName: "testmatrix_semantic.csv",
	kind:     valueKind,
}, {
	sdkKey:   "PKDVCLf-Hq-h-kCzMp-L7Q/uGyK3q9_ckmdxRyI7vjwCw",
	fileName: "testmatrix_number.csv",
	kind:     valueKind,
}, {
	sdkKey:   "PKDVCLf-Hq-h-kCzMp-L7Q/q6jMCFIp-EmuAfnmZhPY7w",
	fileName: "testmatrix_semantic_2.csv",
	kind:     valueKind,
}, {
	sdkKey:   "PKDVCLf-Hq-h-kCzMp-L7Q/qX3TP2dTj06ZpCCT1h_SPA",
	fileName: "testmatrix_sensitive.csv",
	kind:     valueKind,
}, {
	sdkKey:   "PKDVCLf-Hq-h-kCzMp-L7Q/nQ5qkhRAUEa6beEyyrVLBA",
	fileName: "testmatrix_variationId.csv",
	kind:     variationKind,
}}

func TestRolloutIntegration(t *testing.T) {
	for _, test := range integrationTests {
		t.Run(test.fileName, test.runTest)
	}
}

func (test integrationTest) runTest(t *testing.T) {
	var cfg Config
	if os.Getenv("CONFIGCAT_DISABLE_INTEGRATION_TESTS") != "" {
		srv := newConfigServerWithKey(t, test.sdkKey)
		srv.setResponse(configResponse{body: contentForIntegrationTestKey(test.sdkKey)})
		cfg = srv.config()
	}
	tlogger := newTestLogger(t, LogLevelDebug).(*testLogger)
	cfg.Logger = tlogger
	cfg.SDKKey = test.sdkKey
	client := NewCustomClient(cfg)
	defer client.Close()
	err := client.Refresh(context.Background())
	if err != nil {
		t.Fatalf("cannot refresh: %v", err)
	}

	file, fileErr := os.Open(filepath.Join("../resources", test.fileName))
	if fileErr != nil {
		log.Fatal(fileErr)
	}
	defer file.Close()

	reader := csv.NewReader(bufio.NewReader(file))
	reader.Comma = ';'

	header, _ := reader.Read()
	settingKeys := header[4:]
	customKey := header[3]

	lineNumber := 1
	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}
		lineNumber++
		t.Run(fmt.Sprintf("line-%d", lineNumber), func(t *testing.T) {
			var user User
			if line[0] != "##null##" {
				userVal := &UserValue{
					Identifier: nullStr(line[0]),
					Email:      nullStr(line[1]),
					Country:    nullStr(line[2]),
				}
				if s := nullStr(line[3]); s != "" {
					userVal.Custom = map[string]string{
						customKey: s,
					}
				}
				user = userVal
			}
			t.Logf("user %#v", user)

			for i, settingKey := range settingKeys {
				t.Run(fmt.Sprintf("key-%s", settingKey), func(t *testing.T) {
					t.Logf("rule:\n%s", describeRules(client.fetcher.current(), settingKey))
					tlogger.t = t
					var val interface{}
					switch test.kind {
					case valueKind:
						val = client.getValue(settingKey, user)
					case variationKind:
						val = client.VariationID(settingKey, user)
					default:
						t.Fatalf("unexpected kind %v", test.kind)
					}
					expected := line[i+4]
					var expectedVal interface{}
					var err error
					switch val := val.(type) {
					case bool:
						expectedVal, err = strconv.ParseBool(expected)
					case int:
						expectedVal, err = strconv.Atoi(expected)
					case float64:
						expectedVal, err = strconv.ParseFloat(expected, 64)
					case string:
						expectedVal = expected
					default:
						t.Fatalf("value was not handled %T %#v; expected %q", val, val, expected)
					}
					if err != nil {
						t.Fatalf("cannot parse expected value %q as %T: %v", expected, val, err)
					}
					if val != expectedVal {
						t.Errorf("unexpected result for key %s at %s:%d; got %#v want %#v", settingKey, file.Name(), lineNumber, val, expectedVal)
					}
				})
			}
		})
	}
}

func nullStr(s string) string {
	if s == "##null##" {
		return ""
	}
	return s
}

func describeRules(cfg *config, specificKey string) string {
	if cfg == nil {
		return "no config"
	}

	var buf strings.Builder
	printf := func(f string, a ...interface{}) {
		fmt.Fprintf(&buf, f, a...)
	}
	printResult := func(value interface{}, variationID string) {
		printf("\t→ %T %#v; variation %q\n", value, value, variationID)
	}
	keys := make([]string, 0, len(cfg.root.Entries))
	for key := range cfg.root.Entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if specificKey != "" && key != specificKey {
			continue
		}
		entry := cfg.root.Entries[key]
		printf("%q\n", key)
		for _, rule := range entry.RolloutRules {
			printf("\t: user.%s %v %q\n", rule.ComparisonAttribute, rule.Comparator, rule.ComparisonValue)
			printResult(rule.Value, rule.VariationID)
		}
		for _, rule := range entry.PercentageRules {
			printf("\t: %d%%\n", rule.Percentage)
			printResult(rule.Value, rule.VariationID)
		}
		printf("\t: default\n")
		printResult(entry.Value, entry.VariationID)
	}
	return buf.String()
}
