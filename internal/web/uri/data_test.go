package uri

import "testing"

func TestDecode(t *testing.T) {
	cases := []struct{ uri, want string }{
		{"data:image/png;base64,aGVsbG8=", "hello"},
		{"data:image/png;base64,aGVs\n bG8", "hello"}, // wrapped, without padding
		{"DATA:;BASE64,aGVsbG8=", "hello"},
		{"data:image/svg+xml,%3Csvg%2F%3E", "<svg/>"},
		{"data:,a+b", "a+b"},
	}
	for _, test := range cases {
		got, err := DecodeData(test.uri)
		if err != nil || string(got) != test.want {
			t.Errorf("DecodeData(%q) = %q, %v, want %q", test.uri, got, err, test.want)
		}
	}
	for _, uri := range []string{"image.png", "data:image/png;base64", "data:;base64,a$b"} {
		if _, err := DecodeData(uri); err == nil {
			t.Errorf("DecodeData(%q): no error", uri)
		}
	}
}
