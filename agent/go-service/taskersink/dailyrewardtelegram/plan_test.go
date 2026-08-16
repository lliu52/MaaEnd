package dailyrewardtelegram

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDailyRewardEnabledInActiveInstance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mxu-MaaEnd.json")
	data := []byte(`{
        "lastActiveInstanceId": "second",
        "instances": [
            {"id":"first","controllerName":"Win32-Front","tasks":[]},
            {
                "id":"second",
                "controllerName":"Win32-Front",
                "tasks":[{
                    "taskName":"DailyRewards",
                    "enabled":true,
                    "enabledByController":{"Win32-Front":true}
                }]
            }
        ]
    }`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	enabled, found, err := dailyRewardEnabledInFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !found || !enabled {
		t.Fatalf("found=%v enabled=%v", found, enabled)
	}
}

func TestDailyRewardDisabledForController(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mxu-MaaEnd.json")
	data := []byte(`{
        "instances":[{
            "id":"only",
            "controllerName":"Win32-Front",
            "tasks":[{
                "taskName":"DailyRewards",
                "enabled":true,
                "enabledByController":{"Win32-Front":false}
            }]
        }]
    }`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	enabled, found, err := dailyRewardEnabledInFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !found || enabled {
		t.Fatalf("found=%v enabled=%v", found, enabled)
	}
}
