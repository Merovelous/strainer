package tui

import "testing"

func TestBuildOutputName(t *testing.T) {
	tests := []struct {
		input string
		setup func(*filterModel)
		want  string
	}{
		{
			"/path/to/rockyou.txt",
			func(f *filterModel) {
				f.options[0].enabled = true
				f.options[0].value = 8
				f.options[1].enabled = true
				f.options[1].value = 12
			},
			"rockyou_8to12.txt",
		},
		{
			"/path/to/weakpass.txt",
			func(f *filterModel) {
				f.options[0].enabled = true
				f.options[0].value = 6
				f.options[2].enabled = true // ascii
			},
			"weakpass_min6_ascii.txt",
		},
		{
			"/path/to/dump.tar.gz",
			func(f *filterModel) {
				f.options[1].enabled = true
				f.options[1].value = 16
			},
			"dump_max16.txt",
		},
		{
			"/path/to/words.txt",
			func(f *filterModel) {},
			"words_filtered.txt",
		},
		{
			"/path/to/archive.tar.bz2",
			func(f *filterModel) {
				f.options[4].enabled = true // dedup
			},
			"archive_dedup.txt",
		},
	}
	for _, tt := range tests {
		f := newFilterModel(tt.input, 0)
		tt.setup(&f)
		got := f.buildOutputName(tt.input)
		if got != tt.want {
			t.Errorf("buildOutputName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
