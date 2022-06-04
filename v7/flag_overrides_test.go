package configcat

import (
	"context"
	"github.com/configcat/go-sdk/v7/internal/wireconfig"
	qt "github.com/frankban/quicktest"
	"testing"
)

func TestFlagOverrides_File_Simple(t *testing.T) {
	c := qt.New(t)
	cfg := Config{
		FlagOverrides: FlagOverrides{
			FilePath: "../resources/local-simple.json",
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
		FlagOverrides: FlagOverrides{
			FilePath: "../resources/local.json",
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

func TestFlagOverrides_Values_LocalOnly(t *testing.T) {
	c := qt.New(t)
	cfg := Config{
		FlagOverrides: FlagOverrides{
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

func TestFlagOverrides_Values_LocalOverRemote(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponseJSON(rootNodeWithKeyValue("fakeKey", false, wireconfig.BoolEntry))
	cfg := srv.config()

	cfg.FlagOverrides = FlagOverrides{
		Values: map[string]interface{}{
			"fakeKey":     true,
			"nonexisting": true,
		},
		Behaviour: LocalOverRemote,
	}

	client := NewCustomClient(cfg)
	defer client.Close()
	err := client.Refresh(context.Background())
	c.Assert(err, qt.Equals, nil)

	c.Assert(client.GetBoolValue("fakeKey", false, nil), qt.IsTrue)
	c.Assert(client.GetBoolValue("nonexisting", false, nil), qt.IsTrue)
}

func TestFlagOverrides_Values_RemoteOverLocal(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponseJSON(rootNodeWithKeyValue("fakeKey", false, wireconfig.BoolEntry))
	cfg := srv.config()

	cfg.FlagOverrides = FlagOverrides{
		Values: map[string]interface{}{
			"fakeKey":     true,
			"nonexisting": true,
		},
		Behaviour: RemoteOverLocal,
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
	srv.setResponseJSON(rootNodeWithKeyValue("fakeKey", false, wireconfig.BoolEntry))
	cfg := srv.config()

	cfg.FlagOverrides = FlagOverrides{
		Values: map[string]interface{}{
			"fakeKey": true,
			"invalid": BoolFlag{},
		},
		Behaviour: RemoteOverLocal,
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
	srv.setResponseJSON(rootNodeWithKeyValue("fakeKey", false, wireconfig.BoolEntry))
	cfg := srv.config()

	cfg.FlagOverrides = FlagOverrides{
		Values: map[string]interface{}{
			"fakeKey": true,
			"invalid": BoolFlag{},
		},
		Behaviour: LocalOnly,
	}

	client := NewCustomClient(cfg)
	defer client.Close()
	err := client.Refresh(context.Background())
	c.Assert(err, qt.Equals, nil)

	c.Assert(client.GetBoolValue("fakeKey", false, nil), qt.IsFalse)
	c.Assert(client.GetBoolValue("invalid", false, nil), qt.IsFalse)
}
