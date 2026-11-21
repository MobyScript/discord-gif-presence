package main

import "testing"

func TestCheckImage(t *testing.T) {
	ok := []string{"a.png", "b.JPG", "c.jpeg", "d.gif", "e.webp"}
	for _, p := range ok {
		if err := checkImage(p, maxImageSize); err != nil {
			t.Errorf("checkImage(%q) = %v, want nil", p, err)
		}
	}
	if checkImage("a.png", maxImageSize+1) == nil {
		t.Error("an image over 10 MB was accepted")
	}
	if checkImage("notes.txt", 10) == nil {
		t.Error("a .txt file was accepted")
	}
}
