package configcat

import (
	"context"
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestFlagOverrides_File_Simple(t *testing.T) {
	c := qt.New(t)
	cfg := Config{
		FlagOverrides: &FlagOverrides{
			FilePath: "resources/local-simple.json",
		},
	}
	client := NewCustomClient(cfg)
	defer client.Close()

	c.Assert(client.GetBoolValue("enabledFeature", false, nil), qt.IsTrue)
	c.Assert(client.GetBoolValue("disabledFeature", false, nil), qt.IsFalse)
	c.Assert(client.GetIntValue("intSetting", 0, nil), qt.Equals, 5)
	c.Assert(client.GetFloatValue("doubleSetting", 0.0, nil), qt.Equals, 3.14)
	c.Assert(client.GetStringValue("stringSetting", "", nil), qt.Equals, "test")
}

func TestFlagOverrides_File_Complex(t *testing.T) {
	c := qt.New(t)
	cfg := Config{
		FlagOverrides: &FlagOverrides{
			FilePath: "resources/local.json",
		},
	}
	client := NewCustomClient(cfg)
	defer client.Close()

	c.Assert(client.GetBoolValue("enabledFeature", false, nil), qt.IsTrue)
	c.Assert(client.GetBoolValue("disabledFeature", false, nil), qt.IsFalse)
	c.Assert(client.GetIntValue("intSetting", 0, nil), qt.Equals, 5)
	c.Assert(client.Snapshot(nil).GetValue("intSetting"), qt.Equals, 5)
	c.Assert(client.GetFloatValue("doubleSetting", 0.0, nil), qt.Equals, 3.14)
	c.Assert(client.GetStringValue("stringSetting", "", nil), qt.Equals, "test")
}

func TestFlagOverrides_File_Targeting(t *testing.T) {
	c := qt.New(t)
	cfg := Config{
		FlagOverrides: &FlagOverrides{
			FilePath: "resources/local.json",
		},
	}
	client := NewCustomClient(cfg)
	defer client.Close()

	user1 := &UserData{Identifier: "csp@matching.com"}
	c.Assert(client.GetBoolValue("disabledFeature", false, user1), qt.IsTrue)

	user2 := &UserData{Identifier: "csp@notmatching.com"}
	c.Assert(client.GetBoolValue("disabledFeature", false, user2), qt.IsFalse)
}

func TestFlagOverrides_Int_Float(t *testing.T) {
	c := qt.New(t)
	cfg := Config{
		FlagOverrides: &FlagOverrides{
			FilePath: "resources/local-simple.json",
		},
	}
	client := NewCustomClient(cfg)
	defer client.Close()

	c.Assert(client.GetFloatValue("intSetting", 0, nil), qt.Equals, 5.0)
}

func TestFlagOverrides_Values_LocalOnly(t *testing.T) {
	c := qt.New(t)
	cfg := Config{
		FlagOverrides: &FlagOverrides{
			Values: map[string]interface{}{
				"enabledFeature":  true,
				"disabledFeature": false,
				"intSetting":      5,
				"doubleSetting":   3.14,
				"stringSetting":   "test",
			},
		},
	}
	client := NewCustomClient(cfg)
	defer client.Close()

	c.Assert(client.GetBoolValue("enabledFeature", false, nil), qt.IsTrue)
	c.Assert(client.GetBoolValue("disabledFeature", false, nil), qt.IsFalse)
	c.Assert(client.GetIntValue("intSetting", 0, nil), qt.Equals, 5)
	c.Assert(client.GetFloatValue("doubleSetting", 0.0, nil), qt.Equals, 3.14)
	c.Assert(client.GetStringValue("stringSetting", "", nil), qt.Equals, "test")
}

func TestFlagOverrides_Values_Ignored_On_Wrong_Behavior(t *testing.T) {
	c := qt.New(t)
	cfg := Config{
		FlagOverrides: &FlagOverrides{
			Values: map[string]interface{}{
				"enabledFeature":  true,
				"disabledFeature": false,
				"intSetting":      5,
				"doubleSetting":   3.14,
				"stringSetting":   "test",
			},
			Behavior: 5,
		},
	}
	client := NewCustomClient(cfg)
	defer client.Close()

	c.Assert(client.GetBoolValue("enabledFeature", false, nil), qt.IsFalse)
	c.Assert(client.GetBoolValue("disabledFeature", false, nil), qt.IsFalse)
	c.Assert(client.GetIntValue("intSetting", 0, nil), qt.Equals, 0)
	c.Assert(client.GetFloatValue("doubleSetting", 0.0, nil), qt.Equals, 0.0)
	c.Assert(client.GetStringValue("stringSetting", "", nil), qt.Equals, "")
}

func TestFlagOverrides_Values_LocalOverRemote(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponseJSON(rootNodeWithKeyValue("fakeKey", false, BoolSetting))
	cfg := srv.config()

	cfg.FlagOverrides = &FlagOverrides{
		Values: map[string]interface{}{
			"fakeKey":     true,
			"nonexisting": true,
		},
		Behavior: LocalOverRemote,
	}

	client := NewCustomClient(cfg)
	defer client.Close()
	err := client.Refresh(context.Background())
	c.Assert(err, qt.Equals, nil)

	c.Assert(client.GetBoolValue("fakeKey", false, nil), qt.IsTrue)
	c.Assert(client.GetBoolValue("nonexisting", false, nil), qt.IsTrue)
}

func TestFlagOverrides_Values_LocalOverRemoteRespectsRemoteIntType(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponseJSON(rootNodeWithKeyValue("intKey", 5, IntSetting))
	cfg := srv.config()

	// Even though the value has been specified as float locally,
	// the config parser sees the fact that the actual type specified
	// remotely is int, so changes the type locally to correspond.
	// This logic is there so that JSON (which doesn't support int types)
	// keys will work better with int flags.
	cfg.FlagOverrides = &FlagOverrides{
		Values: map[string]interface{}{
			"intKey": 4.0,
		},
		Behavior: LocalOverRemote,
	}

	client := NewCustomClient(cfg)
	defer client.Close()
	err := client.Refresh(context.Background())
	c.Assert(err, qt.Equals, nil)

	c.Assert(client.Snapshot(nil).GetValue("intKey"), qt.Equals, 4)
}

func TestFlagOverrides_Values_RemoteOverLocal(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponseJSON(rootNodeWithKeyValue("fakeKey", false, BoolSetting))
	cfg := srv.config()

	cfg.FlagOverrides = &FlagOverrides{
		Values: map[string]interface{}{
			"fakeKey":     true,
			"nonexisting": true,
		},
		Behavior: RemoteOverLocal,
	}

	client := NewCustomClient(cfg)
	defer client.Close()
	err := client.Refresh(context.Background())
	c.Assert(err, qt.Equals, nil)

	c.Assert(client.GetBoolValue("fakeKey", false, nil), qt.IsFalse)
	c.Assert(client.GetBoolValue("nonexisting", false, nil), qt.IsTrue)
}

func TestFlagOverrides_Values_Remote_Invalid(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponseJSON(rootNodeWithKeyValue("fakeKey", false, BoolSetting))
	cfg := srv.config()

	cfg.FlagOverrides = &FlagOverrides{
		Values: map[string]interface{}{
			"fakeKey": true,
			"invalid": BoolFlag{},
		},
		Behavior: RemoteOverLocal,
	}

	client := NewCustomClient(cfg)
	defer client.Close()
	err := client.Refresh(context.Background())
	c.Assert(err, qt.Equals, nil)

	c.Assert(client.GetBoolValue("fakeKey", false, nil), qt.IsFalse)
	c.Assert(client.GetBoolValue("invalid", false, nil), qt.IsFalse)
}

func TestFlagOverrides_Values_Local_Invalid(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponseJSON(rootNodeWithKeyValue("fakeKey", false, BoolSetting))
	cfg := srv.config()

	cfg.FlagOverrides = &FlagOverrides{
		Values: map[string]interface{}{
			"fakeKey": true,
			"invalid": BoolFlag{},
		},
		Behavior: LocalOnly,
	}

	client := NewCustomClient(cfg)
	defer client.Close()
	err := client.Refresh(context.Background())
	c.Assert(err, qt.Equals, nil)

	c.Assert(client.GetBoolValue("fakeKey", false, nil), qt.IsTrue)
	c.Assert(client.GetBoolValue("invalid", false, nil), qt.IsFalse)
}

func TestPrerequisiteOverride(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body: contentForIntegrationTestKey("configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/JoGwdqJZQ0K2xDy7LnbyOg"),
	})

	tests := []struct {
		key      string
		userId   string
		email    string
		behavior OverrideBehavior
		result   string
	}{
		{"stringDependsOnString", "1", "john@sensitivecompany.com", -1, "Dog"},
		{"stringDependsOnString", "1", "john@sensitivecompany.com", RemoteOverLocal, "Dog"},
		{"stringDependsOnString", "1", "john@sensitivecompany.com", LocalOverRemote, "Dog"},
		{"stringDependsOnString", "1", "john@sensitivecompany.com", LocalOnly, ""},
		{"stringDependsOnString", "2", "john@notsensitivecompany.com", -1, "Cat"},
		{"stringDependsOnString", "2", "john@notsensitivecompany.com", RemoteOverLocal, "Cat"},
		{"stringDependsOnString", "2", "john@notsensitivecompany.com", LocalOverRemote, "Dog"},
		{"stringDependsOnString", "2", "john@notsensitivecompany.com", LocalOnly, ""},
		{"stringDependsOnInt", "1", "john@sensitivecompany.com", -1, "Dog"},
		{"stringDependsOnInt", "1", "john@sensitivecompany.com", RemoteOverLocal, "Dog"},
		{"stringDependsOnInt", "1", "john@sensitivecompany.com", LocalOverRemote, "Cat"},
		{"stringDependsOnInt", "1", "john@sensitivecompany.com", LocalOnly, ""},
		{"stringDependsOnInt", "2", "john@notsensitivecompany.com", -1, "Cat"},
		{"stringDependsOnInt", "2", "john@notsensitivecompany.com", RemoteOverLocal, "Cat"},
		{"stringDependsOnInt", "2", "john@notsensitivecompany.com", LocalOverRemote, "Dog"},
		{"stringDependsOnInt", "2", "john@notsensitivecompany.com", LocalOnly, ""},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test), func(t *testing.T) {
			cfg := srv.config()
			if test.behavior != -1 {
				cfg.FlagOverrides = &FlagOverrides{
					FilePath: "resources/test_override_flagdependency_v6.json",
					Behavior: test.behavior,
				}
			}
			cfg.PollingMode = Manual
			cfg.LogLevel = LogLevelInfo
			client := NewCustomClient(cfg)
			err := client.Refresh(context.Background())
			c.Assert(err, qt.IsNil)

			c.Assert(client.GetStringValue(test.key, "", &UserData{Identifier: test.userId, Email: test.email}), qt.Equals, test.result)

			client.Close()
		})
	}
}

func TestSegmentOverride(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body: contentForIntegrationTestKey("configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/h99HYXWWNE2bH8eWyLAVMA"),
	})

	tests := []struct {
		key      string
		userId   string
		email    string
		behavior OverrideBehavior
		result   interface{}
	}{
		{"developerAndBetaUserSegment", "1", "john@example.com", -1, false},
		{"developerAndBetaUserSegment", "1", "john@example.com", RemoteOverLocal, false},
		{"developerAndBetaUserSegment", "1", "john@example.com", LocalOverRemote, true},
		{"developerAndBetaUserSegment", "1", "john@example.com", LocalOnly, true},
		{"notDeveloperAndNotBetaUserSegment", "2", "kate@example.com", -1, true},
		{"notDeveloperAndNotBetaUserSegment", "2", "kate@example.com", RemoteOverLocal, true},
		{"notDeveloperAndNotBetaUserSegment", "2", "kate@example.com", LocalOverRemote, true},
		{"notDeveloperAndNotBetaUserSegment", "2", "kate@example.com", LocalOnly, nil},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test), func(t *testing.T) {
			cfg := srv.config()
			if test.behavior != -1 {
				cfg.FlagOverrides = &FlagOverrides{
					FilePath: "resources/test_override_segments_v6.json",
					Behavior: test.behavior,
				}
			}
			cfg.PollingMode = Manual
			cfg.LogLevel = LogLevelInfo
			client := NewCustomClient(cfg)
			err := client.Refresh(context.Background())
			c.Assert(err, qt.IsNil)

			c.Assert(client.Snapshot(&UserData{Identifier: test.userId, Email: test.email}).GetValue(test.key), qt.Equals, test.result)

			client.Close()
		})
	}
}
