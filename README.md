# Pixerver

Pixerver is a token-driven image processing service. A client uploads an image
and supplies an input token. Pixerver stores the upload, expands the token into
conversion jobs, creates image variants with ImageMagick, writes artifacts to
configured backends, and posts a callback with the result.

## Current Flow

```text
POST /upload
  -> JWT auth
  -> save multipart file
  -> read token from multipart field or baseToken.json
  -> expand conversionJobs by resolution
  -> encode variants
  -> write each artifact to destinationBackends
  -> POST callbackUrl with artifact metadata
```

## Run Locally

ImageMagick must be installed and available as either `magick` or `convert`.

```sh
export POSTFORM_JWT_SECRET=change-me
export REDIS_ADDR=localhost:6379
go run .
```

Defaults:

- HTTP address: `:8080`
- Upload endpoint: `/upload`
- Health endpoint: `/healthz`
- Upload directory: `uploads`
- Processed directory: `uploads/processed`
- Default token path: `baseToken.json` when present
- Backend config Redis prefix: `backend-configs:`

Useful env vars:

```env
PIXERVER_ADDR=:8080
POSTFORM_UPLOAD_DIR=uploads
PIXERVER_OUTPUT_DIR=uploads/processed
PIXERVER_TOKEN_PATH=baseToken.json
PIXERVER_BACKEND_CONFIG_PREFIX=backend-configs:
POSTFORM_JWT_SECRET=change-me
POSTFORM_JWT_AUDIENCE=
POSTFORM_JWT_ISSUER=
```

## Upload API

`POST /upload` expects multipart form data:

- `file`: image file
- `token`: optional JSON input token. When omitted, `PIXERVER_TOKEN_PATH` is
  used.

Authorization:

```http
Authorization: Bearer <jwt>
```

The response contains the stored source filename/path and, when a token is
available, processing artifacts.

## Input Token

The token shape is demonstrated in [baseToken.json](./baseToken.json).

Important fields:

- `callbackUrl`: best-effort JSON callback target.
- `backends`: map of destination IDs to config tokens.
- `resolutions`: named output sizes. `0x0` means original size.
- `conversionJobs`: output type, resolutions, settings, and destinations.

Backend keys select the implementation. Backend values select the Redis config
token. For multiple buckets of the same provider, use typed aliases:

```json
{
  "backends": {
    "s3": "s3-originals",
    "s3:public": "s3-public",
    "directory": "directory-local"
  }
}
```

`s3` and `s3:public` both use the S3 uploader, but resolve different Redis
config tokens.

## Formats

Supported output types are documented in [FORMATS.md](./FORMATS.md).

```text
jpg, jpeg, webp, avif, png, gif, tiff, bmp, heic, heif, jp2, jxl
```

## Backends

Backend config is documented in [BACKENDS.md](./BACKENDS.md). Implemented
backends:

- `directory`
- `http`
- `s3`
- `gcs`
- `azure`
- `ftp`

Example Redis config:

```sh
redis-cli SET 'backend-configs:directory-local' './public/processed'
redis-cli SET 'backend-configs:s3-public' '{"type":"s3","bucket":"pixerver-public","prefix":"web","region":"ap-south-1"}'
```

## Tests

```sh
GOCACHE=/private/tmp/gocache go test ./...
GOCACHE=/private/tmp/gocache go build ./...
```
