package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/skip2/go-qrcode"
)

func parseQRSize(raw string, fallback int) int {
	size := fallback
	if parsed, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
		size = parsed
	}
	if size < 160 {
		size = 160
	}
	if size > 640 {
		size = 640
	}
	return size
}

func renderQRCodePNG(w http.ResponseWriter, value string, size int) error {
	png, err := qrcode.Encode(value, qrcode.Medium, size)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, err = w.Write(png)
	return err
}
