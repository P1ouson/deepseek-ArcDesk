package control

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSaveImageFileErrorPaths(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := SaveImageFile("missing.png"); err == nil {
		t.Fatal("missing file should fail")
	}
	if err := os.Mkdir("adir", 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveImageFile("adir"); err == nil {
		t.Fatal("directory should fail")
	}
	if err := os.WriteFile("empty.png", nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveImageFile("empty.png"); err == nil {
		t.Fatal("empty file should fail")
	}
	raw := mustBase64(t, tinyPNG)
	if err := os.WriteFile("ok.png", raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if path, err := SaveImageFile("ok.png"); err != nil || path == "" {
		t.Fatalf("SaveImageFile = %q err=%v", path, err)
	}
	huge := make([]byte, maxImageAttachmentBytes+1)
	if err := os.WriteFile("huge.png", huge, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveImageFile("huge.png"); err == nil {
		t.Fatal("oversized image should fail")
	}
}

func TestSaveAttachmentFileErrorPaths(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := SaveAttachmentFile("missing.bin"); err == nil {
		t.Fatal("missing file should fail")
	}
	if err := os.Mkdir("bdir", 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveAttachmentFile("bdir"); err == nil {
		t.Fatal("directory should fail")
	}
	if err := os.WriteFile("empty.bin", nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveAttachmentFile("empty.bin"); err == nil {
		t.Fatal("empty file should fail")
	}
	if err := os.WriteFile("payload.txt", []byte("hello attachment"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := SaveAttachmentFile("payload.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, ".txt") {
		t.Fatalf("path = %q", got)
	}
}

func TestSaveAttachmentDataURLErrorPaths(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := SaveAttachmentDataURL("doc.pdf", "bad"); err == nil {
		t.Fatal("invalid data url")
	}
	if _, err := SaveAttachmentDataURL("doc.pdf", "data:application/pdf;base64,!!!"); err == nil {
		t.Fatal("bad base64")
	}
	if _, err := SaveAttachmentDataURL("doc.pdf", "data:application/pdf;base64,"); err == nil {
		t.Fatal("empty payload")
	}
	got, err := SaveAttachmentDataURL("notes.pdf", "data:application/pdf;base64,JVBERi0=")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, ".pdf") {
		t.Fatalf("path = %q", got)
	}
}

func TestImageDataURLErrorPaths(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := ImageDataURL("missing.png"); err == nil {
		t.Fatal("missing path")
	}
	if err := ensureAttachmentRoot(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".arcdesk/attachments/not-image.txt", []byte("text"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ImageDataURL(".arcdesk/attachments/not-image.txt"); err == nil {
		t.Fatal("non-image should fail")
	}
	if err := os.Mkdir(".arcdesk/attachments/emptydir", 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ImageDataURL(".arcdesk/attachments/emptydir"); err == nil {
		t.Fatal("directory should fail")
	}
}

func TestDetectedImageMimeAndExtVariants(t *testing.T) {
	gif := []byte("GIF89a")
	if got := detectedImageMime(gif); got != "image/gif" {
		t.Fatalf("gif = %q", got)
	}
	if ext := imageExt("image/gif"); ext != ".gif" {
		t.Fatalf("ext = %q", ext)
	}
	if ext := imageExt("image/tiff"); ext != "" {
		t.Fatalf("unknown mime should be empty, got %q", ext)
	}
}

func TestCleanAttachmentPathAndRejectSymlinks(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := ensureAttachmentRoot(); err != nil {
		t.Fatal(err)
	}
	path, err := SaveImageDataURL("data:image/png;base64," + tinyPNG)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cleanAttachmentPath(path); err != nil {
		t.Fatal(err)
	}
	if _, err := cleanAttachmentPath("../../../etc/passwd"); err == nil {
		t.Fatal("traversal should fail")
	}
	root := filepath.Join(".arcdesk", "attachments")
	file := filepath.Join(root, "ok", "f.txt")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := rejectSymlinkComponents(file, root); err != nil {
		t.Fatal(err)
	}
}

func TestSaveClipboardUnsupportedOS(t *testing.T) {
	switch runtime.GOOS {
	case "windows", "darwin", "linux":
		t.Skip("platform has native clipboard handler")
	default:
		if _, err := SaveClipboardImage(); err == nil {
			t.Fatal("expected unsupported error")
		}
	}
}

func TestSaveImageBytesCloseAndWriteErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	raw := mustBase64(t, tinyPNG)
	if _, err := SaveImageBytes("", raw); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveImageBytes("image/png", nil); err == nil {
		t.Fatal("empty bytes")
	}
}

func TestSaveAttachmentFileVariants(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("payload", []byte("hello attachment"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := SaveAttachmentFile("payload")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, ".bin") {
		t.Fatalf("path = %q", got)
	}
	huge := make([]byte, maxFileAttachmentBytes+1)
	if err := os.WriteFile("huge.bin", huge, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveAttachmentFile("huge.bin"); err == nil {
		t.Fatal("oversized attachment should fail")
	}
}

func TestSaveAttachmentDataURLVariants(t *testing.T) {
	t.Chdir(t.TempDir())
	got, err := SaveAttachmentDataURL("payload", "data:application/octet-stream;base64,aGVsbG8=")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, ".bin") {
		t.Fatalf("path = %q", got)
	}
	huge := base64.StdEncoding.EncodeToString(make([]byte, maxFileAttachmentBytes+1))
	if _, err := SaveAttachmentDataURL("big.bin", "data:application/octet-stream;base64,"+huge); err == nil {
		t.Fatal("oversized data url should fail")
	}
}
