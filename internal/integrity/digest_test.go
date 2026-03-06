package integrity

import "testing"

func TestDigestKnownValue(t *testing.T) {
	files := map[string][]byte{
		"a.txt": []byte("hello"),
		"b.txt": []byte("world"),
	}

	d1 := Digest(files)
	d2 := Digest(files)
	if d1 != d2 {
		t.Errorf("Digest not stable: %q != %q", d1, d2)
	}

	if d1[:7] != "sha256-" {
		t.Errorf("Digest should start with 'sha256-', got %q", d1[:7])
	}
}

func TestDigestEmpty(t *testing.T) {
	d := Digest(map[string][]byte{})
	if d[:7] != "sha256-" {
		t.Errorf("Empty digest should start with 'sha256-', got %q", d)
	}
}

func TestDigestOrderIndependence(t *testing.T) {
	files1 := map[string][]byte{
		"z.txt": []byte("last"),
		"a.txt": []byte("first"),
		"m.txt": []byte("middle"),
	}

	files2 := map[string][]byte{
		"a.txt": []byte("first"),
		"m.txt": []byte("middle"),
		"z.txt": []byte("last"),
	}

	d1 := Digest(files1)
	d2 := Digest(files2)
	if d1 != d2 {
		t.Errorf("Digest differs for same content in different insertion order: %q != %q", d1, d2)
	}
}

func TestDigestContentSensitive(t *testing.T) {
	d1 := Digest(map[string][]byte{"a.txt": []byte("hello")})
	d2 := Digest(map[string][]byte{"a.txt": []byte("world")})
	if d1 == d2 {
		t.Error("Different content should produce different digest")
	}
}

func TestDigestPathSensitive(t *testing.T) {
	d1 := Digest(map[string][]byte{"a.txt": []byte("hello")})
	d2 := Digest(map[string][]byte{"b.txt": []byte("hello")})
	if d1 == d2 {
		t.Error("Different file paths with same content should produce different digest")
	}
}

func TestDigestMultipleFilesVsSingle(t *testing.T) {
	single := Digest(map[string][]byte{"a.txt": []byte("helloworld")})
	multi := Digest(map[string][]byte{
		"a.txt": []byte("hello"),
		"b.txt": []byte("world"),
	})

	if single == multi {
		t.Error("Single file and multiple files with same concatenated content should produce different digest")
	}
}

func TestDigestPathIncludedInHash(t *testing.T) {
	// Moving content from one path to another changes the digest
	d1 := Digest(map[string][]byte{"src/a.txt": []byte("hello")})
	d2 := Digest(map[string][]byte{"dst/a.txt": []byte("hello")})
	if d1 == d2 {
		t.Error("Different file paths with same content should produce different digest")
	}
}
