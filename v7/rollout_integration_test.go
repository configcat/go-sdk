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
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"
)

type testKind int

const (
	valueKind     = testKind(0)
	variationKind = testKind(1)
)

type integrationTestSuite struct {
	sdkKey   string
	fileName string
	kind     testKind
}

var integrationTestSuites = []integrationTestSuite{{
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
	t.Parallel()
	integration := os.Getenv("CONFIGCAT_DISABLE_INTEGRATION_TESTS") == ""
	runIntegrationTests(t, integration, LogLevelDebug, func(t *testing.T, test integrationTest) {
		snap := test.client.Snapshot(test.user)
		var val interface{}
		switch test.kind {
		case valueKind:
			val = snap.GetValue(test.key)
		case variationKind:
			val = snap.GetVariationID(test.key)
		default:
			t.Fatalf("unexpected kind %v", test.kind)
		}
		qt.Assert(t, val, qt.Equals, test.expect)
	})
}

func TestRolloutLogLevels(t *testing.T) {
	t.Parallel()
	for _, level := range []LogLevel{
		LogLevelPanic,
		LogLevelFatal,
		LogLevelError,
		LogLevelWarn,
		LogLevelInfo,
		LogLevelDebug,
		LogLevelTrace,
	} {
		level := level
		t.Run(level.String(), func(t *testing.T) {
			t.Parallel()
			runIntegrationTests(t, false, level, func(t *testing.T, test integrationTest) {
				snap := test.client.Snapshot(test.user)
				// Run the test three times concurrently on the same snapshot so we get
				// to test the cached case and concurrent case as well as the uncached case.
				for i := 0; i < 3; i++ {
					var val interface{}
					switch test.kind {
					case valueKind:
						val = snap.GetValue(test.key)
					case variationKind:
						val = snap.GetVariationID(test.key)
					}
					qt.Check(t, val, qt.Equals, test.expect)
				}
			})
		})
	}
}

func TestRolloutConcurrent(t *testing.T) {
	t.Parallel()
	// Test that the caching works OK by running all the tests
	runIntegrationTests(t, false, LogLevelFatal, func(t *testing.T, test integrationTest) {
		// Note: we can't call t.Parallel here because the outer client logger
		// has been bound to this test.
		snap := test.client.Snapshot(test.user)
		// Run the test three times concurrently on the same snapshot so we get
		// to test the cached case and concurrent case as well as the uncached case.
		var wg sync.WaitGroup
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var val interface{}
				switch test.kind {
				case valueKind:
					val = snap.GetValue(test.key)
				case variationKind:
					val = snap.GetVariationID(test.key)
				}
				qt.Check(t, val, qt.Equals, test.expect)
			}()
		}
		wg.Wait()
	})
}

func runIntegrationTests(t *testing.T, integration bool, logLevel LogLevel, runTest func(t *testing.T, test integrationTest)) {
	for _, test := range integrationTestSuites {
		test := test
		t.Run(test.fileName, func(t *testing.T) {
			t.Parallel()
			test.runTests(t, integration, logLevel, runTest)
		})
	}
}

type integrationTest struct {
	// client holds the client to use when querying.
	client *Client
	// user holds the user context for querying the value.
	user User
	// key holds a key to query
	key string
	// kind specifies what type of query to make.
	kind testKind
	// expect holds the expected result
	expect interface{}
}

func (test integrationTestSuite) runTests(t *testing.T, integration bool, logLevel LogLevel, runTest func(t *testing.T, test integrationTest)) {
	var cfg Config
	if !integration {
		srv := newConfigServerWithKey(t, test.sdkKey)
		srv.setResponse(configResponse{body: contentForIntegrationTestKey(test.sdkKey)})
		cfg = srv.config()
	}
	tlogger := newTestLogger(t, logLevel).(*testLogger)
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

	for lineNumber := 2; ; lineNumber++ {
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}
		t.Run(fmt.Sprintf("line-%d", lineNumber), func(t *testing.T) {
			var user User
			if line[0] != "##null##" {
				userVal := &UserData{
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
			if logLevel >= LogLevelInfo {
				t.Logf("user %#v", user)
			}

			for i, settingKey := range settingKeys {
				t.Run(fmt.Sprintf("key-%s", settingKey), func(t *testing.T) {
					if logLevel >= LogLevelInfo {
						t.Logf("rule:\n%s", describeRules(client.fetcher.current(), settingKey))
					}
					tlogger.logFunc = t.Logf
					expected := line[i+4]
					var expectedVal interface{}
					if test.kind == valueKind {
						expectedVal = parseString(expected)
					} else {
						expectedVal = expected
					}
					runTest(t, integrationTest{
						client: client,
						user:   user,
						key:    settingKey,
						kind:   test.kind,
						expect: expectedVal,
					})
				})
			}
		})
	}
}

func parseString(s string) interface{} {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v
	}
	switch s {
	case "True":
		return true
	case "False":
		return false
	}
	return s
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
