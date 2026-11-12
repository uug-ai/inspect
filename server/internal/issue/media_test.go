package issue

import (
	"encoding/base64"
	"testing"
)

func TestSniffMedia(t *testing.T) {
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==")
	ftyp := func(brand string) []byte {
		b := append([]byte{0, 0, 0, 0x18}, []byte("ftyp"+brand+"\x00\x00\x02\x00"+brand+"isom")...)
		return append(b, make([]byte, 32)...)
	}
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"png", png, "image/png"},
		{"jpeg", []byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00"), "image/jpeg"},
		{"webp", []byte("RIFF\x24\x00\x00\x00WEBPVP8 \x18\x00\x00\x00"), "image/webp"},
		{"webm", []byte("\x1A\x45\xDF\xA3\x9F\x42\x86\x81\x01\x42\xF7\x81\x01webm"), "video/webm"},
		{"mp4 mp42", ftyp("mp42"), "video/mp4"},
		{"mp4 iso5 (no mp4 brand)", append([]byte{0, 0, 0, 0x10}, []byte("ftypiso5\x00\x00\x02\x00iso5")...), "video/mp4"},
		{"quicktime rejected", ftyp("qt  "), ""},
		{"heic rejected", ftyp("heic"), ""},
		{"gif rejected", []byte("GIF89a\x01\x00\x01\x00"), ""},
		{"svg rejected", []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), ""},
		{"html rejected", []byte("<!DOCTYPE html><html>"), ""},
		{"text rejected", []byte("hello"), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, ok := SniffMedia(tt.data)
			if got != tt.want || ok != (tt.want != "") {
				t.Errorf("SniffMedia = %q, %v; want %q", got, ok, tt.want)
			}
		})
	}
}
