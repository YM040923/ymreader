package store

import "testing"

func TestToSimplifiedHandlesCommonTitleCharacters(t *testing.T) {
	cases := map[string]string{
		"進擊的巨人":  "进击的巨人",
		"鬼滅之刃":   "鬼灭之刃",
		"咒術迴戰":   "咒术回战",
		"這是一場戰鬥": "这是一场战斗",
		"關於我轉生後": "关于我转生后",
	}
	for input, want := range cases {
		if got := toSimplified(input); got != want {
			t.Fatalf("toSimplified(%q) = %q, want %q", input, got, want)
		}
	}
}
