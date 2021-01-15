package configcat

import (
	"context"
	"testing"
)

func BenchmarkGet(b *testing.B) {
	benchmarks := []struct {
		benchName string
		node      *rootNode
		rule      string
		makeUser  func() User
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
		makeUser: func() User {
			return &UserData{
				Identifier: "unknown-identifier",
				Email:      "x@configcat.com",
				Country:    "United",
			}
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
		makeUser: func() User {
			return &struct {
				Age int
			}{18}
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
		makeUser: func() User {
			return &UserData{
				Identifier: "unknown-identifier",
				Email:      "x@configcat.com",
				Country:    "United",
			}
		},
		want: "high-percent",
	}}
	for _, bench := range benchmarks {
		b.Run(bench.benchName, func(b *testing.B) {
			srv := newConfigServer(b)
			srv.setResponseJSON(bench.node)
			cfg := srv.config()
			cfg.PollingMode = Manual
			cfg.Logger = DefaultLogger(LogLevelError)

			client := NewCustomClient(cfg)
			client.Refresh(context.Background())
			defer client.Close()
			b.Run("get-and-make", func(b *testing.B) {
				val := client.GetStringValue(bench.rule, "", bench.makeUser())
				if val != bench.want {
					b.Fatalf("unexpected result %#v", val)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					client.GetStringValue(bench.rule, "", bench.makeUser())
				}
			})
			b.Run("get-only", func(b *testing.B) {
				rule := String(bench.rule, "")
				snap := client.Snapshot(bench.makeUser())
				val := rule.Get(client.Snapshot(bench.makeUser()))
				if val != bench.want {
					b.Fatalf("unexpected result %#v", val)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					rule.Get(snap)
				}
			})
		})
	}
}
