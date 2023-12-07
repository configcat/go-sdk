package configcat

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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

var integrationTestSuites = []integrationTestSuite{
	{
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
		sdkKey:   "PKDVCLf-Hq-h-kCzMp-L7Q/LcYz135LE0qbcacz2mgXnA",
		fileName: "testmatrix_segments_old.csv",
		kind:     valueKind,
	}, {
		sdkKey:   "PKDVCLf-Hq-h-kCzMp-L7Q/nQ5qkhRAUEa6beEyyrVLBA",
		fileName: "testmatrix_variationId.csv",
		kind:     variationKind,
	},
}

var integrationTestSuitesV2 = []integrationTestSuite{
	{
		sdkKey:   "configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/AG6C1ngVb0CvM07un6JisQ",
		fileName: "testmatrix.csv",
		kind:     valueKind,
	}, {
		sdkKey:   "configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/iV8vH2MBakKxkFZylxHmTg",
		fileName: "testmatrix_semantic.csv",
		kind:     valueKind,
	}, {
		sdkKey:   "configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw",
		fileName: "testmatrix_number.csv",
		kind:     valueKind,
	}, {
		sdkKey:   "configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/U8nt3zEhDEO5S2ulubCopA",
		fileName: "testmatrix_semantic_2.csv",
		kind:     valueKind,
	}, {
		sdkKey:   "configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/-0YmVOUNgEGKkgRF-rU65g",
		fileName: "testmatrix_sensitive.csv",
		kind:     valueKind,
	}, {
		sdkKey:   "configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/y_ZB7o-Xb0Swxth-ZlMSeA",
		fileName: "testmatrix_segments_old.csv",
		kind:     valueKind,
	}, {
		sdkKey:   "configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/ByMO9yZNn02kXcm72lnY1A",
		fileName: "testmatrix_and_or.csv",
		kind:     valueKind,
	}, {
		sdkKey:   "configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ",
		fileName: "testmatrix_comparators_v6.csv",
		kind:     valueKind,
	}, {
		sdkKey:   "configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/JoGwdqJZQ0K2xDy7LnbyOg",
		fileName: "testmatrix_prerequisite_flag.csv",
		kind:     valueKind,
	}, {
		sdkKey:   "configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/h99HYXWWNE2bH8eWyLAVMA",
		fileName: "testmatrix_segments.csv",
		kind:     valueKind,
	}, {
		sdkKey:   "configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/Da6w8dBbmUeMUBhh0iEeQQ",
		fileName: "testmatrix_unicode.csv",
		kind:     valueKind,
	}, {
		sdkKey:   "configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/spQnkRTIPEWVivZkWM84lQ",
		fileName: "testmatrix_variationId.csv",
		kind:     variationKind,
	},
}

func TestRolloutIntegration(t *testing.T) {
	t.Parallel()
	integration := os.Getenv("CONFIGCAT_DISABLE_INTEGRATION_TESTS") == ""
	testFunc := func(t *testing.T, test integrationTest) {
		snap := test.client.Snapshot(test.user)
		var val interface{}
		switch test.kind {
		case valueKind:
			val = snap.GetValue(test.key)
		case variationKind:
			details := snap.GetValueDetails(test.key)
			val = details.Data.VariationID
		default:
			t.Fatalf("unexpected kind %v", test.kind)
		}
		qt.Assert(t, val, qt.Equals, test.expect)
	}
	runIntegrationTests(t, integration, LogLevelWarn, integrationTestSuites, testFunc)
	runIntegrationTests(t, integration, LogLevelWarn, integrationTestSuitesV2, testFunc)
}

func TestRolloutConcurrent(t *testing.T) {
	t.Parallel()
	testFunc := func(t *testing.T, test integrationTest) {
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
					details := snap.GetValueDetails(test.key)
					val = details.Data.VariationID
				}
				qt.Check(t, val, qt.Equals, test.expect)
			}()
		}
		wg.Wait()
	}
	// Test that the caching works OK by running all the tests
	runIntegrationTests(t, false, LogLevelWarn, integrationTestSuites, testFunc)
	runIntegrationTests(t, false, LogLevelWarn, integrationTestSuitesV2, testFunc)
}

func TestGenerateFiles(t *testing.T) {
	t.Skip("this test only generates the content of 'resources/content-by-key.json'")
	generateJsonContentFile("resources/content-by-key.json", append(integrationTestSuites, integrationTestSuitesV2...))
}

func runIntegrationTests(t *testing.T, integration bool, logLevel LogLevel, suite []integrationTestSuite, runTest func(t *testing.T, test integrationTest)) {
	for _, test := range suite {
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
	tlogger := newTestLogger(t).(*testLogger)
	cfg.Logger = tlogger
	cfg.LogLevel = logLevel
	cfg.SDKKey = test.sdkKey
	client := NewCustomClient(cfg)
	defer client.Close()
	err := client.Refresh(context.Background())
	if err != nil {
		t.Fatalf("cannot refresh: %v", err)
	}

	file, fileErr := os.Open(filepath.Join("resources", test.fileName))
	if fileErr != nil {
		log.Fatal(fileErr)
	}
	defer file.Close()

	reader := csv.NewReader(bufio.NewReader(file))
	reader.Comma = ';'
	reader.LazyQuotes = true

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
					userVal.Custom = map[string]interface{}{
						customKey: s,
					}
				}
				user = userVal
			}
			if logLevel <= LogLevelInfo {
				t.Logf("user %#v", user)
			}

			for i, settingKey := range settingKeys {
				t.Run(fmt.Sprintf("key-%s", settingKey), func(t *testing.T) {
					tlogger.t = t
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

func generateJsonContentFile(fileName string, suite []integrationTestSuite) {
	cont := make(map[string]interface{}, len(suite))
	for _, test := range suite {
		resp, err := http.DefaultClient.Get("https://cdn-global.configcat.com/configuration-files/" + test.sdkKey + "/config_v6.json")
		if err != nil {
			panic(err)
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		var a map[string]interface{}
		err = json.Unmarshal(b, &a)
		if err != nil {
			panic(err)
		}
		cont[test.sdkKey] = a
		resp.Body.Close()
	}
	d, err := json.Marshal(cont)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile(fileName, d, fs.ModeExclusive)
	if err != nil {
		panic(err)
	}
}
