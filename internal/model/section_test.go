package model

import (
	"reflect"
	"testing"
)

func TestParsePageRanges(t *testing.T) {
	tests := []struct {
		input   string
		want    []PageRange
		wantErr bool
	}{
		{"all", nil, false},
		{"ALL", nil, false},
		{"", nil, false},
		{"1-50", []PageRange{{1, 50}}, false},
		{"1", []PageRange{{1, 1}}, false},
		{"1,5,10-20", []PageRange{{1, 1}, {5, 5}, {10, 20}}, false},
		{"3-5", []PageRange{{3, 5}}, false},
		{"1-2, 5-8, 100", []PageRange{{1, 2}, {5, 8}, {100, 100}}, false},
		// Error cases
		{"abc", nil, true},
		{"1-abc", nil, true},
		{"abc-5", nil, true},
		{"50-10", nil, true}, // first > last
		{"0-5", nil, true},   // page < 1
		{"-1-5", nil, true},  // negative
	}
	for _, tt := range tests {
		got, err := ParsePageRanges(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParsePageRanges(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ParsePageRanges(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestExpandPages(t *testing.T) {
	tests := []struct {
		ranges []PageRange
		want   []int
	}{
		{nil, nil},
		{[]PageRange{{1, 3}}, []int{1, 2, 3}},
		{[]PageRange{{1, 1}, {5, 5}, {10, 12}}, []int{1, 5, 10, 11, 12}},
	}
	for _, tt := range tests {
		got := ExpandPages(tt.ranges)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ExpandPages(%v) = %v, want %v", tt.ranges, got, tt.want)
		}
	}
}

func TestPageInRanges(t *testing.T) {
	tests := []struct {
		page   int
		ranges []PageRange
		want   bool
	}{
		{5, nil, true}, // nil = "all"
		{5, []PageRange{{1, 10}}, true},
		{15, []PageRange{{1, 10}}, false},
		{5, []PageRange{{1, 3}, {5, 5}}, true},
		{4, []PageRange{{1, 3}, {5, 5}}, false},
	}
	for _, tt := range tests {
		got := PageInRanges(tt.page, tt.ranges)
		if got != tt.want {
			t.Errorf("PageInRanges(%d, %v) = %v, want %v", tt.page, tt.ranges, got, tt.want)
		}
	}
}

func TestPageRangeContains(t *testing.T) {
	pr := PageRange{First: 10, Last: 20}
	tests := []struct {
		page int
		want bool
	}{
		{9, false},
		{10, true},
		{15, true},
		{20, true},
		{21, false},
	}
	for _, tt := range tests {
		if got := pr.Contains(tt.page); got != tt.want {
			t.Errorf("PageRange{10,20}.Contains(%d) = %v, want %v", tt.page, got, tt.want)
		}
	}
}
