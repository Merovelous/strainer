package main

import "testing"

// Captured `7z l` output samples for different 7-Zip versions and content types.
// Tests must not require the 7z binary — all inputs are inline fixtures.

// fixture7zV2102 is output from 7-Zip 21.02 on a simple archive.
const fixture7zV2102 = `
7-Zip 21.02 alpha (x64) : Copyright (c) 1999-2021 Igor Pavlov : 2021-05-06
p7zip Version 21.02 alpha (locale=en_US.UTF-8,Utf16=on,HugeFiles=on,64 bits,8 CPUs x64)

Scanning the drive for archives:
1 file, 3824646 bytes (3735 KiB)

Listing archive: weakpass.7z

--
Path = weakpass.7z
Type = 7z
Physical Size = 3824646
Headers Size = 162
Method = LZMA2:24
Solid = -
Blocks = 1

   Date      Time    Attr         Size   Compressed  Name
------------------- ----- ------------ ------------  ------------------------
2021-05-01 12:00:00 ....A     12345678      3824484  rockyou.txt
------------------- ----- ------------ ------------  ------------------------
2021-05-01 12:00:00             12345678      3824484  1 files
`

// fixture7zV2301 simulates 7-Zip 23.01 output with multiple files.
const fixture7zV2301 = `
7-Zip 23.01 (x64) : Copyright (c) 1999-2023 Igor Pavlov : 2023-06-20

Scanning the drive for archives:
1 file, 5000000 bytes (4883 KiB)

Listing archive: wordlists.7z

--
Path = wordlists.7z
Type = 7z

   Date      Time    Attr         Size   Compressed  Name
------------------- ----- ------------ ------------  ------------------------
2023-01-01 00:00:00 ....A      1000000       400000  rockyou.txt
2023-01-01 00:00:00 ....A      2000000       800000  weakpass.txt
2023-01-01 00:00:00 ....A       500000       200000  subdir/passwords.txt
------------------- ----- ------------ ------------  ------------------------
2023-01-01 00:00:00            3500000      1400000  3 files, 0 folders
`

// fixture7zUnicode has filenames with Unicode characters and deep paths.
const fixture7zUnicode = `
7-Zip 21.07 (x64) : Copyright (c) 1999-2021 Igor Pavlov : 2021-12-26

Listing archive: multi.7z

--
Path = multi.zip
Type = zip

   Date      Time    Attr         Size   Compressed  Name
------------------- ----- ------------ ------------  ------------------------
2022-03-15 08:30:00 ....A       123456        50000  wordlists/пароли.txt
2022-03-15 08:30:00 ....A       234567        90000  wordlists/密码列表.txt
2022-03-15 08:30:00 ....A        12345         5000  a/b/c/d/deep_path.txt
------------------- ----- ------------ ------------  ------------------------
2022-03-15 08:30:00              370368       145000  3 files
`

// fixture7zNoFiles is an archive with no file entries.
const fixture7zNoFiles = `
7-Zip 23.01 (x64)

Listing archive: empty.7z

--
Path = empty.7z
Type = 7z

   Date      Time    Attr         Size   Compressed  Name
------------------- ----- ------------ ------------  ------------------------
------------------- ----- ------------ ------------  ------------------------
0 files, 0 folders
`

// fixturep7zip simulates p7zip-style output (different column widths).
const fixturep7zip = `
p7zip Version 16.02 (locale=en_US.UTF-8,Utf16=on,HugeFiles=on,64 bits,2 CPUs Intel(R))

Listing archive: archive.zip

--
Path = archive.zip
Type = zip

   Date      Time    Attr         Size   Compressed  Name
------------------- ----- ------------ ------------  ------------------------
2020-01-01 00:00:00 .....       999999       400000  file one.txt
2020-01-01 00:00:00 .....      1000000       400001  file_two.txt
------------------- ----- ------------ ------------  ------------------------
`

func TestParse7zListing_SingleFile(t *testing.T) {
	entries := parse7zListing(fixture7zV2102)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %v", len(entries), entries)
	}
	if entries[0].name != "rockyou.txt" {
		t.Errorf("entry[0].name = %q, want %q", entries[0].name, "rockyou.txt")
	}
	if entries[0].index != 0 {
		t.Errorf("entry[0].index = %d, want 0", entries[0].index)
	}
}

func TestParse7zListing_MultipleFiles(t *testing.T) {
	entries := parse7zListing(fixture7zV2301)
	want := []string{"rockyou.txt", "weakpass.txt", "subdir/passwords.txt"}
	if len(entries) != len(want) {
		t.Fatalf("expected %d entries, got %d: %v", len(want), len(entries), entries)
	}
	for i, name := range want {
		if entries[i].name != name {
			t.Errorf("entry[%d].name = %q, want %q", i, entries[i].name, name)
		}
		if entries[i].index != i {
			t.Errorf("entry[%d].index = %d, want %d", i, entries[i].index, i)
		}
	}
}

func TestParse7zListing_UnicodeFilenames(t *testing.T) {
	entries := parse7zListing(fixture7zUnicode)
	want := []string{"wordlists/пароли.txt", "wordlists/密码列表.txt", "a/b/c/d/deep_path.txt"}
	if len(entries) != len(want) {
		t.Fatalf("expected %d entries, got %d: %v", len(want), len(entries), entries)
	}
	for i, name := range want {
		if entries[i].name != name {
			t.Errorf("entry[%d].name = %q, want %q", i, entries[i].name, name)
		}
	}
}

func TestParse7zListing_EmptyArchive(t *testing.T) {
	entries := parse7zListing(fixture7zNoFiles)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty archive, got %d: %v", len(entries), entries)
	}
}

func TestParse7zListing_p7zipFormat(t *testing.T) {
	entries := parse7zListing(fixturep7zip)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(entries), entries)
	}
	if entries[0].name != "file one.txt" {
		t.Errorf("entry[0].name = %q, want %q", entries[0].name, "file one.txt")
	}
	if entries[1].name != "file_two.txt" {
		t.Errorf("entry[1].name = %q, want %q", entries[1].name, "file_two.txt")
	}
}

func TestParse7zListing_Empty(t *testing.T) {
	if entries := parse7zListing(""); entries != nil {
		t.Errorf("expected nil for empty input, got %v", entries)
	}
	if entries := parse7zListing("no separator line here\njust text\n"); entries != nil {
		t.Errorf("expected nil for input with no separator, got %v", entries)
	}
}

func TestIsSepLine(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"------------------- ----- ------------ ------------  ------------------------", true},
		{"----------", true},
		{"--- --- ---", true},
		{"", false},
		{"------", false},   // too short (< 10)
		{"abc-------", false}, // has non-dash, non-space chars before dashes
		{"hello", false},
	}
	for _, tt := range tests {
		if got := isSepLine(tt.line); got != tt.want {
			t.Errorf("isSepLine(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestIsSummaryLine(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"1 files, 0 folders", true},
		{"3 files", true},
		{"0 files, 0 folders", true},
		{"files and folders here", false}, // doesn't start with digit
		{"", false},
		{"some file path.txt", false},
	}
	for _, tt := range tests {
		if got := isSummaryLine(tt.line); got != tt.want {
			t.Errorf("isSummaryLine(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}
