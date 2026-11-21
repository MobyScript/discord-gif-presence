package main

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxImageSize = 10 << 20

var imageTypes = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true}

func checkImage(path string, size int64) error {
	if !imageTypes[strings.ToLower(filepath.Ext(path))] {
		return fmt.Errorf("%s is not a PNG, JPG, GIF or WEBP file.", filepath.Base(path))
	}
	if size > maxImageSize {
		return fmt.Errorf("%s is larger than 10 MB.", filepath.Base(path))
	}
	return nil
}

// uploadImage puts a local image on catbox.moe, because Discord can only
// show images from a public link. It returns that link.
func uploadImage(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	if err := checkImage(path, st.Size()); err != nil {
		return "", err
	}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	w.WriteField("reqtype", "fileupload")
	part, _ := w.CreateFormFile("fileToUpload", filepath.Base(path))
	if _, err := io.Copy(part, f); err != nil {
		return "", err
	}
	w.Close()

	resp, err := (&http.Client{Timeout: time.Minute}).Post("https://catbox.moe/user/api.php", w.FormDataContentType(), &body)
	if err != nil {
		return "", fmt.Errorf("The upload to catbox.moe failed: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	link := strings.TrimSpace(string(b))
	if resp.StatusCode != 200 || !strings.HasPrefix(link, "https://") {
		return "", fmt.Errorf("The upload to catbox.moe failed: %s %s", resp.Status, link)
	}
	return link, nil
}
