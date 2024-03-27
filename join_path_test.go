package arc

import "testing"

func Test_JoinPath(t *testing.T) {
	tests := []struct {
		name, url, path, result string
	}{
		{
			name:   "slash on url",
			url:    "https://miner.test/arc/",
			path:   "v1/txs",
			result: "https://miner.test/arc/v1/txs",
		},
		{
			name:   "slashes on path",
			url:    "https://miner.test/arc",
			path:   "/v1/txs",
			result: "https://miner.test/arc/v1/txs",
		},
		{
			name:   "slashes on both sides",
			url:    "https://miner.test/arc/",
			path:   "/v1/txs",
			result: "https://miner.test/arc/v1/txs",
		},
		{
			name:   "no middle slashes",
			url:    "https://miner.test/arc",
			path:   "v1/txs",
			result: "https://miner.test/arc/v1/txs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := JoinPath(tt.url, tt.path)
			if err != nil {
				t.Fatalf("Failed to join path : %s", err)
			}

			t.Logf("Result : %s", result)
			if result != tt.result {
				t.Errorf("Wrong result url : \n   got : %s\n  want : %s", result, tt.result)
			}
		})
	}
}
