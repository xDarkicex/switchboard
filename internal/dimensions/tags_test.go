package dimensions

import "testing"

func TestTagsMatch(t *testing.T) {
	tags := Tags{Length: "short", Style: "cinematic", Refs: "image"}

	if !tags.Match(map[string]string{"length": "short"}) {
		t.Error("should match exact length")
	}
	if !tags.Match(map[string]string{"style": "cinematic"}) {
		t.Error("should match exact style")
	}
	if tags.Match(map[string]string{"length": "long"}) {
		t.Error("should not match wrong length")
	}
	if tags.Match(map[string]string{"refs": "video"}) {
		t.Error("should not match wrong refs")
	}
	if !tags.Match(map[string]string{"camera": "static"}) {
		t.Error("missing dimension should match")
	}
	if !tags.Match(map[string]string{}) {
		t.Error("empty criteria should match")
	}
}

func TestTagsEqual(t *testing.T) {
	a := Tags{Length: "short", Style: "cinematic"}
	b := Tags{Length: "short", Style: "cinematic"}
	c := Tags{Length: "short", Style: "photorealistic"}
	if !a.Equal(b) {
		t.Error("equal tags should match")
	}
	if a.Equal(c) {
		t.Error("different style should not match")
	}
}

func TestTagsSetGet(t *testing.T) {
	tags := Tags{}
	tags.Set("length", "medium")
	tags.Set("style", "cinematic")
	if tags.Get("length") != "medium" {
		t.Errorf("Get length = %q", tags.Get("length"))
	}
	if tags.Get("style") != "cinematic" {
		t.Errorf("Get style = %q", tags.Get("style"))
	}
	if tags.Get("unknown") != "" {
		t.Error("unknown dimension should be empty")
	}
}
