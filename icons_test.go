package main

import "testing"

func TestIconLookupPrecedence(t *testing.T) {
	is := newIconSet(hopConfig{})
	// full filename beats extension: go.mod has ext "mod" but a filename entry
	if is.file("go.mod") != builtinFileIcons["go.mod"]+" " {
		t.Error("filename entry should win over extension")
	}
	if is.file("main.go") != builtinFileIcons["go"]+" " {
		t.Error("extension lookup failed")
	}
	if is.file("MAIN.GO") != builtinFileIcons["go"]+" " {
		t.Error("lookup should be case-insensitive")
	}
	if is.file("mystery.xyz") != iconFileDefault+" " {
		t.Error("unknown extension should fall back to generic file icon")
	}
	if is.dir("node_modules") != builtinDirIcons["node_modules"]+" " {
		t.Error("special directory lookup failed")
	}
	if is.dir("some-random-dir") != iconDirDefault+" " {
		t.Error("unknown directory should fall back to generic folder icon")
	}
}

func TestIconUserOverrides(t *testing.T) {
	cfg := hopConfig{Icons: iconOverrides{
		Dirs:  map[string]string{"Widgets": "W"},
		Files: map[string]string{"go": "G", "special.txt": "S"},
	}}
	is := newIconSet(cfg)
	if is.dir("widgets") != "W " {
		t.Error("user dir override (lowercased) not applied")
	}
	if is.file("main.go") != "G " {
		t.Error("user extension override not applied")
	}
	if is.file("special.txt") != "S " {
		t.Error("user filename override not applied")
	}
	if is.file("other.txt") != builtinFileIcons["txt"]+" " {
		t.Error("builtin mappings should survive user overrides")
	}
}
