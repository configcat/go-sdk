package configcat

import (
	"context"
	"fmt"
	"testing"

	"github.com/configcat/go-sdk/v7/internal/wireconfig"
	qt "github.com/frankban/quicktest"
)

func BenchmarkGet(b *testing.B) {
	benchmarks := []struct {
		benchName      string
		node           *wireconfig.RootNode
		rule           string
		makeUser       func() User
		want           string
		setDefaultUser bool
	}{{
		benchName: "one-of",
		node: &wireconfig.RootNode{
			Entries: map[string]*wireconfig.Entry{
				"rule": {
					VariationID: "607147d5",
					Value:       "no-match",
					RolloutRules: []*wireconfig.RolloutRule{{
						ComparisonAttribute: "Email",
						ComparisonValue:     "a@configcat.com, b@configcat.com",
						Comparator:          wireconfig.OpOneOf,
						VariationID:         "385d9803",
						Value:               "email-match",
					}, {
						ComparisonAttribute: "Country",
						ComparisonValue:     "United",
						Comparator:          wireconfig.OpNotOneOf,
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
		benchName: "one-of-with-default-user",
		node: &wireconfig.RootNode{
			Entries: map[string]*wireconfig.Entry{
				"rule": {
					VariationID: "607147d5",
					Value:       "no-match",
					RolloutRules: []*wireconfig.RolloutRule{{
						ComparisonAttribute: "Email",
						ComparisonValue:     "a@configcat.com, b@configcat.com",
						Comparator:          wireconfig.OpOneOf,
						VariationID:         "385d9803",
						Value:               "email-match",
					}, {
						ComparisonAttribute: "Country",
						ComparisonValue:     "United",
						Comparator:          wireconfig.OpNotOneOf,
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
		setDefaultUser: true,
		want:           "no-match",
	}, {
		benchName: "less-than-with-int",
		node: &wireconfig.RootNode{
			Entries: map[string]*wireconfig.Entry{
				"rule": {
					VariationID: "607147d5",
					Value:       "no-match",
					RolloutRules: []*wireconfig.RolloutRule{{
						ComparisonAttribute: "Age",
						ComparisonValue:     "21",
						Comparator:          wireconfig.OpLessNum,
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
		node: &wireconfig.RootNode{
			Entries: map[string]*wireconfig.Entry{
				"bool30TrueAdvancedRules": {
					VariationID: "607147d5",
					Value:       "no-match",
					RolloutRules: []*wireconfig.RolloutRule{{
						ComparisonAttribute: "Email",
						ComparisonValue:     "a@configcat.com, b@configcat.com",
						Comparator:          wireconfig.OpOneOf,
						VariationID:         "385d9803",
						Value:               "email-match",
					}, {
						ComparisonAttribute: "Country",
						ComparisonValue:     "United",
						Comparator:          wireconfig.OpNotOneOf,
						VariationID:         "385d9803",
						Value:               "country-match",
					}},
					PercentageRules: []*wireconfig.PercentageRule{{
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
	}, {
		benchName: "no-rules",
		node: &wireconfig.RootNode{
			Entries: map[string]*wireconfig.Entry{
				"simple": {
					Value: "no-match",
				},
			},
		},
		rule: "simple",
		makeUser: func() User {
			return &UserData{
				Identifier: "unknown-identifier",
				Email:      "x@configcat.com",
				Country:    "United",
			}
		},
		want: "no-match",
	}, {
		benchName: "no-user",
		node: &wireconfig.RootNode{
			Entries: map[string]*wireconfig.Entry{
				"bool30TrueAdvancedRules": {
					VariationID: "607147d5",
					Value:       "no-match",
					RolloutRules: []*wireconfig.RolloutRule{{
						ComparisonAttribute: "Email",
						ComparisonValue:     "a@configcat.com, b@configcat.com",
						Comparator:          wireconfig.OpOneOf,
						VariationID:         "385d9803",
						Value:               "email-match",
					}, {
						ComparisonAttribute: "Country",
						ComparisonValue:     "United",
						Comparator:          wireconfig.OpNotOneOf,
						VariationID:         "385d9803",
						Value:               "country-match",
					}},
					PercentageRules: []*wireconfig.PercentageRule{{
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
			return nil
		},
		want: "no-match",
	}}
	for _, bench := range benchmarks {
		b.Run(bench.benchName, func(b *testing.B) {
			srv := newConfigServer(b)
			srv.setResponseJSON(bench.node)
			cfg := srv.config()
			cfg.PollingMode = Manual
			cfg.Logger = DefaultLogger(LogLevelError)
			user := bench.makeUser()
			if bench.setDefaultUser {
				cfg.DefaultUser = user
			}

			client := NewCustomClient(cfg)
			client.Refresh(context.Background())
			defer client.Close()
			cfg.DefaultUser =
				b.Run("get-and-make", func(b *testing.B) {
					rule := String(bench.rule, "")
					val := rule.Get(client.Snapshot(user))
					if val != bench.want {
						b.Fatalf("unexpected result %#v", val)
					}
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						rule.Get(client.Snapshot(user))
					}
				})
			b.Run("get-only", func(b *testing.B) {
				rule := String(bench.rule, "")
				snap := client.Snapshot(user)
				val := rule.Get(snap)
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

func BenchmarkNewSnapshot(b *testing.B) {
	c := qt.New(b)
	b.ReportAllocs()
	const nkeys = 100
	m := make(map[string]interface{})
	for i := 0; i < nkeys; i++ {
		m[fmt.Sprint("key", i)] = false
	}
	logger := newTestLogger(c, LogLevelError)
	for i := 0; i < b.N; i++ {
		NewSnapshot(logger, m)
	}
}
