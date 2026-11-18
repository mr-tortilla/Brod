package subscription

import "testing"

func TestParseUserInfo(t *testing.T) {
	got := parseUserInfo("upload=12; download=345; total=1000; expire=1893456000")
	if got == nil || *got != (Info{Upload: 12, Download: 345, Total: 1000, Expire: 1893456000}) {
		t.Fatalf("%+v", got)
	}
	if got := parseUserInfo("upload=1.5e3;download=2;Total=0;EXPIRE=0;foo=bar"); got == nil || got.Upload != 1500 || got.Download != 2 {
		t.Fatalf("дробные числа и регистр: %+v", got)
	}
	for _, h := range []string{"", "  ", "foo=bar", "garbage"} {
		if got := parseUserInfo(h); got != nil {
			t.Fatalf("%q: ожидался nil, получено %+v", h, got)
		}
	}
}
