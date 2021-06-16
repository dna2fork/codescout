package ui

import (
	"fmt"
	"reflect"
	"testing"
)

func TestFindLineRangeInQueryParameters(t *testing.T) {
	tests := []struct {
		name            string
		queryParameters map[string][]string
		wantLineRange   *lineRange
	}{
		{name: "empty parameters", queryParameters: map[string][]string{}, wantLineRange: nil},
		{name: "single line", queryParameters: map[string][]string{"L123": {}}, wantLineRange: &lineRange{StartLine: 123}},
		{name: "line range", queryParameters: map[string][]string{"L10-123": {}}, wantLineRange: &lineRange{StartLine: 10, EndLine: 123}},
		{name: "invalid range", queryParameters: map[string][]string{"L-123": {}}, wantLineRange: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := findLineRangeInQueryParameters(test.queryParameters)
			if !reflect.DeepEqual(test.wantLineRange, got) {
				t.Errorf("got %v, want %v", got, test.wantLineRange)
			}
		})
	}
}

func TestGetBlobPreviewImageURL(t *testing.T) {
	previewServiceURL := "https://preview.sourcegraph.com"
	blobURLPath := "/github.com/sourcegraph/sourcegraph/-/blob/client/browser/src/end-to-end/github.test.ts"
	tests := []struct {
		name      string
		lineRange *lineRange
		wantURL   string
	}{
		{name: "empty line range", lineRange: nil, wantURL: fmt.Sprintf("%s%s", previewServiceURL, blobURLPath)},
		{name: "single line", lineRange: &lineRange{StartLine: 123}, wantURL: fmt.Sprintf("%s%s?range=L123", previewServiceURL, blobURLPath)},
		{name: "line range", lineRange: &lineRange{StartLine: 123, EndLine: 125}, wantURL: fmt.Sprintf("%s%s?range=L123-125", previewServiceURL, blobURLPath)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := getBlobPreviewImageURL(previewServiceURL, blobURLPath, test.lineRange)
			if !reflect.DeepEqual(test.wantURL, got) {
				t.Errorf("got %v, want %v", got, test.wantURL)
			}
		})
	}
}

func TestGetBlobPreviewTitle(t *testing.T) {
	tests := []struct {
		name         string
		lineRange    *lineRange
		blobFilePath string
		wantTitle    string
	}{
		{name: "empty line range", lineRange: nil, blobFilePath: "path/a.txt", wantTitle: "a.txt"},
		{name: "single line", lineRange: &lineRange{StartLine: 4}, blobFilePath: "path/a.txt", wantTitle: "a.txt#L4"},
		{name: "line range", lineRange: &lineRange{StartLine: 1, EndLine: 10}, blobFilePath: "path/a.txt", wantTitle: "a.txt#L1-10"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := getBlobPreviewTitle(test.blobFilePath, test.lineRange)
			if !reflect.DeepEqual(test.wantTitle, got) {
				t.Errorf("got %v, want %v", got, test.wantTitle)
			}
		})
	}
}
