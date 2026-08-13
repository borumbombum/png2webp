# png2webp

Convert PNG images to WebP from the command line — single file or a whole
directory at once.

Written in pure Go — just `go build`, no system dependencies to install.

## Why this exists

WebP isn't always smaller than PNG. Lossless WebP can lose to PNG on noisy
or photo-like images, and it's easy to end up quietly shipping a _bigger_
file than you started with. This tool avoids that:

- By default it encodes **both** lossless and lossy versions of each PNG
  and keeps whichever is smaller.
- If neither beats the original PNG size, **it leaves the file alone** —
  no `.webp` is written, and it tells you why. (You can override this
  with `-force` if you really want the WebP anyway.)

## Setup

```bash
go mod init png2webp
go get github.com/KarpelesLab/gowebp
go build -o png2webp .
```

## Usage

### Convert a single file

```bash
png2webp -in photo.png -quality 85
# -> writes photo.webp next to it, using quality 85 for the lossy candidate
```

```bash
png2webp -in photo.png -out assets/photo.webp -quality 85
# -> custom output path
```

### Convert every PNG in a directory

```bash
png2webp -dir ./images
```

Each `some.png` in the folder becomes `some.webp` right next to it,
non-recursively.

### Example output

```
$ png2webp -dir ./images
  images/icon.png: 1007 B -> 518 B (48.6% smaller, lossless)
  images/photo.png: 245.4 KB -> 50.8 KB (79.3% smaller, lossy)
  images/noise.png: kept as PNG — lossy WebP (88.2 KB) wasn't smaller than the original (88.2 KB); use -force to write anyway
done: 2 converted, 1 skipped
```

## Flags

| Flag               | Default     | Description                                                     |
| ------------------ | ----------- | --------------------------------------------------------------- |
| `-in <file>`       | —           | Convert a single PNG file                                       |
| `-out <file>`      | `<in>.webp` | Output path for `-in` (ignored with `-dir`)                     |
| `-dir <folder>`    | —           | Convert every `.png` in a folder                                |
| `-quality <0-100>` | `90`        | Quality used for the lossy candidate                            |
| `-lossless`        | off         | Skip the comparison, force lossless only                        |
| `-lossy`           | off         | Skip the comparison, force lossy only                           |
| `-force`           | off         | Write the `.webp` even if it ends up bigger than the source PNG |

`-lossless` and `-lossy` are mutually exclusive. If neither is set
(the default), the tool tries both and keeps the smaller result.

## More examples

```bash
# Force lossy encoding at a specific quality (skips the lossless comparison)
png2webp -in photo.png -lossy -quality 85

# Force pixel-perfect lossless (e.g. for icons/screenshots where exact colors matter)
png2webp -in icon.png -lossless

# Batch-convert a folder, always writing output even if bigger than the PNG
png2webp -dir ./images -force
```

## Notes

- Only top-level `.png` files in a `-dir` are processed (no recursion into subfolders).
- Lossless encoding is pixel-perfect — a round-trip decode matches the source exactly.
- Non-PNG files passed to `-in` are rejected with a clear error.
