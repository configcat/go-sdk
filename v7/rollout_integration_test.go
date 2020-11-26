package configcat

import (
	"bufio"
	"encoding/csv"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

const (
	valueKind     = 0
	variationKind = 1
)

type integrationTest struct {
	sdkKey   string
	fileName string
	mode     RefreshMode
	kind     int
}

func BenchmarkGetValue(b *testing.B) {
	b.ReportAllocs()
	logger := DefaultLogger(LogLevelError)
	client := NewCustomClient(integrationTests[0].sdkKey, ClientConfig{
		Logger:         logger,
		Mode:           ManualPoll(),
		StaticLogLevel: true,
	})
	client.Refresh()
	defer client.Close()
	user := NewUser("unknown-identifier")
	val := client.GetValueForUser("bool30TrueAdvancedRules", "default", user)
	if val != false {
		b.Fatalf("unexpected result %#v", val)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.GetValueForUser("bool30TrueAdvancedRules", "default", user)
	}
}

var integrationTests = []integrationTest{{
	sdkKey:   "PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A",
	fileName: "testmatrix.csv",
	mode:     AutoPoll(120 * time.Second),
	kind:     valueKind,
}, {
	sdkKey:   "PKDVCLf-Hq-h-kCzMp-L7Q/BAr3KgLTP0ObzKnBTo5nhA",
	fileName: "testmatrix_semantic.csv",
	mode:     LazyLoad(120*time.Second, false),
	kind:     valueKind,
}, {
	sdkKey:   "PKDVCLf-Hq-h-kCzMp-L7Q/uGyK3q9_ckmdxRyI7vjwCw",
	fileName: "testmatrix_number.csv",
	mode:     ManualPoll(),
	kind:     valueKind,
}, {
	sdkKey:   "PKDVCLf-Hq-h-kCzMp-L7Q/q6jMCFIp-EmuAfnmZhPY7w",
	fileName: "testmatrix_semantic_2.csv",
	mode:     AutoPoll(120 * time.Second),
	kind:     valueKind,
}, {
	sdkKey:   "PKDVCLf-Hq-h-kCzMp-L7Q/qX3TP2dTj06ZpCCT1h_SPA",
	fileName: "testmatrix_sensitive.csv",
	mode:     AutoPoll(120 * time.Second),
	kind:     valueKind,
}, {
	sdkKey:   "PKDVCLf-Hq-h-kCzMp-L7Q/nQ5qkhRAUEa6beEyyrVLBA",
	fileName: "testmatrix_variationId.csv",
	mode:     AutoPoll(120 * time.Second),
	kind:     variationKind,
}}

func TestRolloutIntegration(t *testing.T) {
	for _, test := range integrationTests {
		t.Run(test.fileName, test.runTest)
	}
}

func (test integrationTest) runTest(t *testing.T) {
	var cfg ClientConfig
	if os.Getenv("CONFIGCAT_DISABLE_INTEGRATION_TESTS") != "" {
		srv := newConfigServerWithKey(t, test.sdkKey)
		srv.setResponse(configResponse{body: contentForIntegrationTestKey(test.sdkKey)})
		cfg = srv.config()
	}
	cfg.Mode = test.mode
	cfg.StaticLogLevel = true
	cfg.Logger = newTestLogger(t, LogLevelError)
	client := NewCustomClient(test.sdkKey, cfg)
	client.Refresh()
	defer client.Close()

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
		var user *User
		if line[0] != "##null##" {
			identifier := line[0]
			email := nullStr(line[1])
			country := nullStr(line[2])
			custom := map[string]string{}
			if s := nullStr(line[3]); s != "" {
				custom[customKey] = line[3]
			}
			user = NewUserWithAdditionalAttributes(identifier, email, country, custom)
		}

		for i, settingKey := range settingKeys {
			var val interface{}
			switch test.kind {
			case valueKind:
				val = client.GetValueForUser(settingKey, nil, user)
			case variationKind:
				val = client.GetVariationIdForUser(settingKey, "", user)
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
				t.Fatalf("Value was not handled %#v", val)
			}
			if err != nil {
				t.Fatalf("cannot parse expected value %q as %T: %v", expected, val, err)
			}
			if val != expectedVal {
				t.Errorf("unexpected result for key %s at %s:%d; got %#v want %#v", settingKey, file.Name(), lineNumber, val, expectedVal)
				for key, val := range user.attributes {
					t.Logf("user %s: %q", key, val)
				}
			}
		}
	}
}

func nullStr(s string) string {
	if s == "##null##" {
		return ""
	}
	return s
}
