# Backend Configuration

`baseToken.json` keeps backend values as lookup keys. The backend map key
selects the backend implementation. The backend map value is the Redis config
token where credentials/settings live.

At runtime Pixerver first checks Redis for the config token, then falls back to
environment variables and direct JSON/URL configs for local testing.

By default a value such as `s3-originals` resolves from Redis key:

```text
backend-configs:s3-originals
```

This means one upload token can target multiple buckets of the same provider:

```json
{
  "backends": {
    "s3": "s3-originals",
    "s3:public": "s3-public",
    "gcs": "gcs-archive"
  },
  "conversionJobs": [
    {
      "type": "webp",
      "resolutions": ["thumbnail"],
      "destinationBackends": ["s3:public", "gcs"]
    }
  ]
}
```

The map key is also the destination ID used by `destinationBackends`. The part
before `:`, `.`, or `/` selects the backend type. So `s3`, `s3:public`, and
`s3.archive` all use the S3 uploader, but can point to different Redis config
tokens and therefore different buckets, prefixes, credentials, regions, or
endpoints.

The Redis prefix defaults to `backend-configs:` and can be changed with:

```env
PIXERVER_BACKEND_CONFIG_PREFIX=backend-configs:
```

Examples using `redis-cli`:

```sh
redis-cli SET 'backend-configs:s3-originals' '{"type":"s3","bucket":"pixerver-originals","prefix":"raw","region":"ap-south-1"}'
redis-cli SET 'backend-configs:s3-public' '{"type":"s3","bucket":"pixerver-public","prefix":"web","region":"ap-south-1"}'
redis-cli SET 'backend-configs:gcs-archive' '{"type":"gcs","bucket":"pixerver-archive","prefix":"images"}'
```

You can still put direct JSON or URL configs in the token while testing.

## Directory

```env
redis-cli SET 'backend-configs:directory-local' './public/processed'
```

## HTTP

```env
redis-cli SET 'backend-configs:http-webhook' 'https://example.com/upload'
```

## S3

Uses the AWS default credential chain unless `accessKeyId` and
`secretAccessKey` are provided. `endpoint` and `usePathStyle` support
S3-compatible providers such as MinIO or R2.

```env
redis-cli SET 'backend-configs:s3-public' '{"type":"s3","bucket":"my-bucket","prefix":"images","region":"ap-south-1","endpoint":"","accessKeyId":"","secretAccessKey":"","sessionToken":"","usePathStyle":false}'
```

URL form:

```text
s3://my-bucket/images
```

## Google Cloud Storage

Uses Application Default Credentials unless `credentialsFile` or
`credentialsJSON` is provided.

```env
redis-cli SET 'backend-configs:gcs-archive' '{"type":"gcs","bucket":"my-bucket","prefix":"images","credentialsFile":"","credentialsJSON":""}'
```

URL forms:

```text
gs://my-bucket/images
gcs://my-bucket/images
```

## Azure Blob Storage

Use `connectionString`, or `accountName` plus `accountKey`.

```env
redis-cli SET 'backend-configs:azure-backup' '{"type":"azure","container":"images","prefix":"variants","connectionString":"","accountName":"","accountKey":""}'
```

URL forms:

```text
azure://container-name/variants
az://container-name/variants
```

URL forms still need credentials from JSON for real uploads.

## FTP

Set `explicitTLS=true` for FTPES, or `tls=true` for implicit FTPS.

```env
redis-cli SET 'backend-configs:ftp-partner' '{"type":"ftp","host":"ftp.example.com","port":21,"username":"user","password":"pass","remoteDir":"uploads","tls":false,"explicitTLS":false,"insecureSkipVerify":false}'
```

URL forms:

```text
ftp://user:pass@example.com:21/uploads
ftps://user:pass@example.com:990/uploads
```
