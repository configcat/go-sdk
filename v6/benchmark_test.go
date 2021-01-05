package configcat

import (
	"strconv"
	"testing"
)

func BenchmarkGet(b *testing.B) {
	benchmarks := []struct {
		benchName string
		node      *rootNode
		rule      string
		makeUser  func() *User
		want      string
	}{{
		benchName: "one-of",
		node: &rootNode{
			Entries: map[string]*entry{
				"rule": {
					VariationID: "607147d5",
					Value:       "no-match",
					RolloutRules: []*rolloutRule{{
						ComparisonAttribute: "Email",
						ComparisonValue:     "a@configcat.com, b@configcat.com",
						Comparator:          opOneOf,
						VariationID:         "385d9803",
						Value:               "email-match",
					}, {
						ComparisonAttribute: "Country",
						ComparisonValue:     "United",
						Comparator:          opNotOneOf,
						VariationID:         "385d9803",
						Value:               "country-match",
					}},
				},
			},
		},
		rule: "rule",
		makeUser: func() *User {
			return NewUserWithAdditionalAttributes("unknown-identifier", "x@configcat.com", "United", nil)
		},
		want: "no-match",
	}, {
		benchName: "less-than-with-int",
		node: &rootNode{
			Entries: map[string]*entry{
				"rule": {
					VariationID: "607147d5",
					Value:       "no-match",
					RolloutRules: []*rolloutRule{{
						ComparisonAttribute: "Age",
						ComparisonValue:     "21",
						Comparator:          opLessNum,
						VariationID:         "385d9803",
						Value:               "age-match",
					}},
				},
			},
		},
		rule: "rule",
		makeUser: func() *User {
			age := 18
			return NewUserWithAdditionalAttributes("unknown-identifier", "x@configcat.com", "United", map[string]string{
				"Age": strconv.FormatInt(int64(age), 10),
			})
		},
		want: "age-match",
	}, {
		benchName: "with-percentage",
		node: &rootNode{
			Entries: map[string]*entry{
				"bool30TrueAdvancedRules": {
					VariationID: "607147d5",
					Value:       "no-match",
					RolloutRules: []*rolloutRule{{
						ComparisonAttribute: "Email",
						ComparisonValue:     "a@configcat.com, b@configcat.com",
						Comparator:          opOneOf,
						VariationID:         "385d9803",
						Value:               "email-match",
					}, {
						ComparisonAttribute: "Country",
						ComparisonValue:     "United",
						Comparator:          opNotOneOf,
						VariationID:         "385d9803",
						Value:               "country-match",
					}},
					PercentageRules: []percentageRule{{
						VariationID: "607147d5",
						Value:       "low-percent",
						Percentage:  30,
					}, {
						VariationID: "385d9803",
						Value:       "high-percent",
						Percentage:  70,
					}},
				},
			},
		},
		rule: "bool30TrueAdvancedRules",
		makeUser: func() *User {
			return NewUserWithAdditionalAttributes("unknown-identifier", "x@configcat.com", "United", nil)
		},
		want: "high-percent",
	}}
	for _, bench := range benchmarks {
		b.Run(bench.benchName, func(b *testing.B) {
			b.ReportAllocs()
			srv := newConfigServer(b)
			srv.setResponseJSON(bench.node)
			cfg := srv.config()
			cfg.Mode = ManualPoll()
			cfg.Logger = DefaultLogger(LogLevelError)
			cfg.StaticLogLevel = true

			client := NewCustomClient(srv.sdkKey(), cfg)
			client.Refresh()
			defer client.Close()
			val := client.GetValueForUser(bench.rule, "", bench.makeUser()).(string)
			if val != bench.want {
				b.Fatalf("unexpected result %#v", val)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = client.GetValueForUser(bench.rule, "", bench.makeUser()).(string)
			}
		})
	}
}
