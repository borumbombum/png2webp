// png2webp converts PNG images to WebP using a pure-Go WebP encoder
// (github.com/KarpelesLab/gowebp). No CGO, no libwebp, no external
// binaries required — just `go build`.
//
// For every PNG it tries both lossless and lossy encoding and keeps
// whichever is smaller. If neither beats the original PNG size, it
// leaves that file alone (no .webp written) rather than ever producing
// a file bigger than the source.
//
// Setup:
//
//	go mod init png2webp
//	go get github.com/KarpelesLab/gowebp
//
// Usage:
//
//	go run main.go -in photo.png -out photo.webp
//	go run main.go -dir ./images     // convert every .png in a folder
//
//	go run main.go -dir ./images -lossy -quality 85
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/KarpelesLab/gowebp"
)

const defaultLossyQuality = 90 // high quality; lossless is also tried and the smaller of the two wins

func main() {
	inPath := flag.String("in", "", "input PNG file")
	outPath := flag.String("out", "", "output WebP file (default: same name, .webp extension)")
	dir := flag.String("dir", "", "convert all .png files in this directory")
	quality := flag.Float64("quality", defaultLossyQuality, "WebP quality, 0-100 (used for the lossy candidate)")
	lossless := flag.Bool("lossless", false, "force lossless encoding only (skip the auto best-of comparison)")
	lossy := flag.Bool("lossy", false, "force lossy encoding only (skip the auto best-of comparison)")
	force := flag.Bool("force", false, "write the .webp even if it ends up bigger than the source PNG")
	flag.Parse()

	m := modeAuto
	switch {
	case *lossless && *lossy:
		fmt.Fprintln(os.Stderr, "error: -lossless and -lossy are mutually exclusive")
		os.Exit(1)
	case *lossless:
		m = modeLossless
	case *lossy:
		m = modeLossy
	}

	switch {
	case *dir != "":
		if err := convertDir(*dir, float32(*quality), m, *force); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case *inPath != "":
		out := *outPath
		if out == "" {
			out = strings.TrimSuffix(*inPath, filepath.Ext(*inPath)) + ".webp"
		}
		if _, err := convertFile(*inPath, out, float32(*quality), m, *force); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: png2webp -in file.png -out file.webp   (or)   png2webp -dir ./images")
		flag.PrintDefaults()
		os.Exit(1)
	}
}

// decodePNG opens and decodes a PNG file into an image.Image.
func decodePNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s does not look like a valid image: %w", path, err)
	}
	if format != "png" {
		return nil, fmt.Errorf("%s is a %s file, not a PNG", path, format)
	}
	return img, nil
}

type mode int

const (
	modeAuto mode = iota // try both, keep the smaller
	modeLossless
	modeLossy
)

// bestWebP encodes img according to the mode and returns the bytes plus a
// label ("lossless" or "lossy") for reporting.
func bestWebP(img image.Image, quality float32, m mode) ([]byte, string, error) {
	if m == modeLossless {
		var buf bytes.Buffer
		if err := gowebp.Encode(&buf, img, nil); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "lossless", nil
	}
	if m == modeLossy {
		var buf bytes.Buffer
		if err := gowebp.Encode(&buf, img, &gowebp.Options{Lossy: true, Quality: quality, Method: 4}); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "lossy", nil
	}

	// modeAuto: try both, keep the smaller.
	var llBuf, lyBuf bytes.Buffer
	errLL := gowebp.Encode(&llBuf, img, nil)
	errLY := gowebp.Encode(&lyBuf, img, &gowebp.Options{Lossy: true, Quality: quality, Method: 4})

	switch {
	case errLL != nil && errLY != nil:
		return nil, "", errLL
	case errLL != nil:
		return lyBuf.Bytes(), "lossy", nil
	case errLY != nil:
		return llBuf.Bytes(), "lossless", nil
	case lyBuf.Len() < llBuf.Len():
		return lyBuf.Bytes(), "lossy", nil
	default:
		return llBuf.Bytes(), "lossless", nil
	}
}

// convertFile converts a single PNG to WebP. Returns true if a .webp was
// written (skipped by default if WebP wouldn't be smaller than the PNG,
// unless force is set).
func convertFile(in, out string, quality float32, m mode, force bool) (bool, error) {
	img, err := decodePNG(in)
	if err != nil {
		return false, err
	}

	inInfo, err := os.Stat(in)
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", in, err)
	}
	origSize := inInfo.Size()

	data, label, err := bestWebP(img, quality, m)
	if err != nil {
		return false, fmt.Errorf("encoding %s: %w", in, err)
	}
	newSize := int64(len(data))

	if newSize >= origSize && !force {
		fmt.Printf("  %s: kept as PNG — %s WebP (%s) wasn't smaller than the original (%s); use -force to write anyway\n",
			in, label, formatBytes(newSize), formatBytes(origSize))
		return false, nil
	}

	if err := os.WriteFile(out, data, 0644); err != nil {
		return false, fmt.Errorf("writing %s: %w", out, err)
	}

	pct := (1 - float64(newSize)/float64(origSize)) * 100
	fmt.Printf("  %s: %s -> %s (%.1f%% smaller, %s)\n",
		in, formatBytes(origSize), formatBytes(newSize), pct, label)
	return true, nil
}

// formatBytes renders a byte count as a short human-readable string.
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.1f %s", float64(n)/float64(div), units[exp])
}

// convertDir converts every *.png file in a directory (non-recursive).
func convertDir(dir string, quality float32, m mode, force bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading dir %s: %w", dir, err)
	}

	converted, skipped := 0, 0
	for _, e := range entries {
		if e.IsDir() || strings.ToLower(filepath.Ext(e.Name())) != ".png" {
			continue
		}
		in := filepath.Join(dir, e.Name())
		out := strings.TrimSuffix(in, filepath.Ext(in)) + ".webp"
		wrote, err := convertFile(in, out, quality, m, force)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skipping %s: %v\n", in, err)
			skipped++
			continue
		}
		if wrote {
			converted++
		} else {
			skipped++
		}
	}
	fmt.Printf("done: %d converted, %d skipped\n", converted, skipped)
	return nil
}