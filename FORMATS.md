# Formats

Pixerver uses ImageMagick for image conversion. The service registers these
output types:

```text
jpg, jpeg, webp, avif, png, gif, tiff, bmp, heic, heif, jp2, jxl
```

`jpeg` and `jpe` normalize to `jpg`; `tif` normalizes to `tiff`.

Actual support for HEIC, HEIF, JP2, JXL, and AVIF depends on the delegates
compiled into the ImageMagick installation on the host.

## Common Settings

All formats accept these settings:

```json
{
  "quality": "80",
  "strip": "true",
  "resizeMode": "fit",
  "ignoreAspect": "false",
  "flatten": "false",
  "background": "white"
}
```

Resolution width/height are supplied by the named resolution in the input token.

`resizeMode` values:

- `fit`: fit inside the requested box while preserving aspect ratio. This is the
  default ImageMagick behavior.
- `fill`: cover the requested box, center-crop, and extent to the exact
  requested width/height.

Set `ignoreAspect` to `true` only when a stretched output is acceptable.

## Format-Specific Settings

JPEG:

```json
{
  "quality": "80",
  "progressive": "true",
  "optimize": "true",
  "strip": "true"
}
```

WebP:

```json
{
  "quality": "75",
  "lossless": "false",
  "method": "5"
}
```

AVIF:

```json
{
  "quality": "55",
  "effort": "6"
}
```

PNG:

```json
{
  "compressionLevel": "9",
  "strip": "true"
}
```

GIF:

```json
{
  "delay": "10",
  "loop": "0"
}
```

TIFF:

```json
{
  "compression": "LZW"
}
```
