package configcat

import (
	"context"
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
)

func BenchmarkGet(b *testing.B) {
	age := float64(21)
	benchmarks := []struct {
		benchName      string
		node           *ConfigJson
		rule           string
		makeUser       func() User
		want           string
		setDefaultUser bool
	}{{
		benchName: "one-of",
		node: &ConfigJson{
			Settings: map[string]*Setting{
				"rule": {
					Type:        StringSetting,
					VariationID: "607147d5",
					Value:       &SettingValue{StringValue: "no-match"},
					TargetingRules: []*TargetingRule{{
						Conditions: []*Condition{{
							UserCondition: &UserCondition{
								ComparisonAttribute: "Email",
								StringArrayValue:    []string{"a@configcat.com", "b@configcat.com"},
								Comparator:          OpOneOf,
							},
						}},
						ServedValue: &ServedValue{
							Value:       &SettingValue{StringValue: "email-match"},
							VariationID: "385d9803",
						},
					}, {
						Conditions: []*Condition{{
							UserCondition: &UserCondition{
								ComparisonAttribute: "Country",
								StringArrayValue:    []string{"United"},
								Comparator:          OpNotOneOf,
							},
						}},
						ServedValue: &ServedValue{
							Value:       &SettingValue{StringValue: "country-match"},
							VariationID: "385d9803",
						},
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
		node: &ConfigJson{
			Settings: map[string]*Setting{
				"rule": {
					Type:        StringSetting,
					VariationID: "607147d5",
					Value:       &SettingValue{StringValue: "no-match"},
					TargetingRules: []*TargetingRule{{
						Conditions: []*Condition{{
							UserCondition: &UserCondition{
								ComparisonAttribute: "Email",
								StringArrayValue:    []string{"a@configcat.com", "b@configcat.com"},
								Comparator:          OpOneOf,
							},
						}},
						ServedValue: &ServedValue{
							Value:       &SettingValue{StringValue: "email-match"},
							VariationID: "385d9803",
						},
					}, {
						Conditions: []*Condition{{
							UserCondition: &UserCondition{
								ComparisonAttribute: "Country",
								StringArrayValue:    []string{"United"},
								Comparator:          OpNotOneOf,
							},
						}},
						ServedValue: &ServedValue{
							Value:       &SettingValue{StringValue: "country-match"},
							VariationID: "385d9803",
						},
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
		node: &ConfigJson{
			Settings: map[string]*Setting{
				"rule": {
					Type:        StringSetting,
					VariationID: "607147d5",
					Value:       &SettingValue{StringValue: "no-match"},
					TargetingRules: []*TargetingRule{{
						Conditions: []*Condition{{
							UserCondition: &UserCondition{
								ComparisonAttribute: "Age",
								DoubleValue:         &age,
								Comparator:          OpLessNum,
							},
						}},
						ServedValue: &ServedValue{
							Value:       &SettingValue{StringValue: "age-match"},
							VariationID: "385d9803",
						},
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
		node: &ConfigJson{
			Settings: map[string]*Setting{
				"bool30TrueAdvancedRules": {
					Type:        StringSetting,
					VariationID: "607147d5",
					Value:       &SettingValue{StringValue: "no-match"},
					TargetingRules: []*TargetingRule{{
						Conditions: []*Condition{{
							UserCondition: &UserCondition{
								ComparisonAttribute: "Email",
								StringArrayValue:    []string{"a@configcat.com", "b@configcat.com"},
								Comparator:          OpOneOf,
							},
						}},
						ServedValue: &ServedValue{
							Value:       &SettingValue{StringValue: "email-match"},
							VariationID: "385d9803",
						},
					}, {
						Conditions: []*Condition{{
							UserCondition: &UserCondition{
								ComparisonAttribute: "Country",
								StringArrayValue:    []string{"United"},
								Comparator:          OpNotOneOf,
							},
						}},
						ServedValue: &ServedValue{
							Value:       &SettingValue{StringValue: "country-match"},
							VariationID: "385d9803",
						},
					}},
					PercentageOptions: []*PercentageOption{{
						Percentage:  30,
						Value:       &SettingValue{StringValue: "low-percent"},
						VariationID: "607147d5",
					}, {
						Percentage:  70,
						Value:       &SettingValue{StringValue: "high-percent"},
						VariationID: "385d9803",
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
		node: &ConfigJson{
			Settings: map[string]*Setting{
				"simple": {
					Type:  StringSetting,
					Value: &SettingValue{StringValue: "no-match"},
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
		node: &ConfigJson{
			Settings: map[string]*Setting{
				"bool30TrueAdvancedRules": {
					Type:        StringSetting,
					VariationID: "607147d5",
					Value:       &SettingValue{StringValue: "no-match"},
					TargetingRules: []*TargetingRule{{
						Conditions: []*Condition{{
							UserCondition: &UserCondition{
								ComparisonAttribute: "Email",
								StringArrayValue:    []string{"a@configcat.com", "b@configcat.com"},
								Comparator:          OpOneOf,
							},
						}},
						ServedValue: &ServedValue{
							Value:       &SettingValue{StringValue: "email-match"},
							VariationID: "385d9803",
						},
					}, {
						Conditions: []*Condition{{
							UserCondition: &UserCondition{
								ComparisonAttribute: "Country",
								StringArrayValue:    []string{"United"},
								Comparator:          OpNotOneOf,
							},
						}},
						ServedValue: &ServedValue{
							Value:       &SettingValue{StringValue: "country-match"},
							VariationID: "385d9803",
						},
					}},
					PercentageOptions: []*PercentageOption{{
						Percentage:  30,
						Value:       &SettingValue{StringValue: "low-percent"},
						VariationID: "607147d5",
					}, {
						Percentage:  70,
						Value:       &SettingValue{StringValue: "high-percent"},
						VariationID: "385d9803",
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
			cfg.Logger = DefaultLogger()
			cfg.LogLevel = LogLevelError
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
	logger := newTestLogger(c)
	for i := 0; i < b.N; i++ {
		NewSnapshot(logger, m)
	}
}
