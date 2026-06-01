package pipeline

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jlaffaye/ftp"
	"google.golang.org/api/option"
)

type backendConfig struct {
	Type string `json:"type"`

	Bucket string `json:"bucket"`
	Prefix string `json:"prefix"`
	Region string `json:"region"`

	Endpoint        string `json:"endpoint"`
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	SessionToken    string `json:"sessionToken"`
	UsePathStyle    bool   `json:"usePathStyle"`

	ProjectID       string `json:"projectId"`
	CredentialsFile string `json:"credentialsFile"`
	CredentialsJSON string `json:"credentialsJSON"`

	AccountName      string `json:"accountName"`
	AccountKey       string `json:"accountKey"`
	ConnectionString string `json:"connectionString"`
	Container        string `json:"container"`

	Host               string `json:"host"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	RemoteDir          string `json:"remoteDir"`
	TLS                bool   `json:"tls"`
	ExplicitTLS        bool   `json:"explicitTLS"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify"`
}

func (p Processor) storeExternal(ctx context.Context, kind, spec, artifactPath string) (string, error) {
	cfg, err := parseExternalBackendConfig(kind, spec)
	if err != nil {
		return "", err
	}
	switch cfg.Type {
	case "s3":
		return uploadS3(ctx, cfg, artifactPath)
	case "gcs":
		return uploadGCS(ctx, cfg, artifactPath)
	case "azure":
		return uploadAzure(ctx, cfg, artifactPath)
	case "ftp":
		return uploadFTP(ctx, cfg, artifactPath)
	default:
		return "", fmt.Errorf("unsupported external backend type %q", cfg.Type)
	}
}

func parseExternalBackendConfig(defaultKind, spec string) (backendConfig, error) {
	var cfg backendConfig
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return cfg, errors.New("empty backend config")
	}
	if strings.HasPrefix(spec, "{") {
		if err := json.Unmarshal([]byte(spec), &cfg); err != nil {
			return cfg, err
		}
		if cfg.Type == "" {
			cfg.Type = defaultKind
		}
		return cfg, nil
	}
	u, err := url.Parse(spec)
	if err == nil && u.Scheme != "" {
		return backendConfigFromURL(defaultKind, u), nil
	}
	return cfg, fmt.Errorf("backend %s requires JSON config or URL", defaultKind)
}

func backendConfigFromURL(defaultKind string, u *url.URL) backendConfig {
	cfg := backendConfig{Type: defaultKind}
	switch u.Scheme {
	case "s3":
		cfg.Type = "s3"
		cfg.Bucket = u.Host
		cfg.Prefix = strings.TrimPrefix(u.Path, "/")
	case "gs", "gcs":
		cfg.Type = "gcs"
		cfg.Bucket = u.Host
		cfg.Prefix = strings.TrimPrefix(u.Path, "/")
	case "az", "azure":
		cfg.Type = "azure"
		cfg.Container = u.Host
		cfg.Prefix = strings.TrimPrefix(u.Path, "/")
	case "ftp", "ftps":
		cfg.Type = "ftp"
		cfg.Host = u.Hostname()
		cfg.RemoteDir = strings.TrimPrefix(u.Path, "/")
		cfg.TLS = u.Scheme == "ftps"
		if p := u.Port(); p != "" {
			cfg.Port, _ = strconv.Atoi(p)
		}
		if u.User != nil {
			cfg.Username = u.User.Username()
			cfg.Password, _ = u.User.Password()
		}
	}
	return cfg
}

func uploadS3(ctx context.Context, cfg backendConfig, artifactPath string) (string, error) {
	if cfg.Bucket == "" {
		return "", errors.New("s3 backend requires bucket")
	}
	key := objectKey(cfg.Prefix, artifactPath)
	opts := []func(*config.LoadOptions) error{}
	if cfg.Region != "" {
		opts = append(opts, config.WithRegion(cfg.Region))
	}
	if cfg.AccessKeyID != "" || cfg.SecretAccessKey != "" {
		if cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
			return "", errors.New("s3 static credentials require accessKeyId and secretAccessKey")
		}
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken)))
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return "", err
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.UsePathStyle
	})
	file, err := os.Open(artifactPath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return "", err
	}
	return "s3://" + cfg.Bucket + "/" + key, nil
}

func uploadGCS(ctx context.Context, cfg backendConfig, artifactPath string) (string, error) {
	if cfg.Bucket == "" {
		return "", errors.New("gcs backend requires bucket")
	}
	opts := []option.ClientOption{}
	if cfg.CredentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(cfg.CredentialsFile))
	}
	if cfg.CredentialsJSON != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(cfg.CredentialsJSON)))
	}
	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return "", err
	}
	defer client.Close()

	key := objectKey(cfg.Prefix, artifactPath)
	file, err := os.Open(artifactPath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	writer := client.Bucket(cfg.Bucket).Object(key).NewWriter(ctx)
	if _, err := io.Copy(writer, file); err != nil {
		_ = writer.Close()
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	return "gs://" + cfg.Bucket + "/" + key, nil
}

func uploadAzure(ctx context.Context, cfg backendConfig, artifactPath string) (string, error) {
	if cfg.Container == "" {
		return "", errors.New("azure backend requires container")
	}
	var client *azblob.Client
	var err error
	if cfg.ConnectionString != "" {
		client, err = azblob.NewClientFromConnectionString(cfg.ConnectionString, nil)
	} else {
		if cfg.AccountName == "" || cfg.AccountKey == "" {
			return "", errors.New("azure backend requires connectionString or accountName/accountKey")
		}
		cred, credErr := azblob.NewSharedKeyCredential(cfg.AccountName, cfg.AccountKey)
		if credErr != nil {
			return "", credErr
		}
		endpoint := cfg.Endpoint
		if endpoint == "" {
			endpoint = fmt.Sprintf("https://%s.blob.core.windows.net/", cfg.AccountName)
		}
		client, err = azblob.NewClientWithSharedKeyCredential(endpoint, cred, nil)
	}
	if err != nil {
		return "", err
	}
	blobName := objectKey(cfg.Prefix, artifactPath)
	file, err := os.Open(artifactPath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := client.UploadFile(ctx, cfg.Container, blobName, file, nil); err != nil {
		return "", err
	}
	return "azure://" + cfg.Container + "/" + blobName, nil
}

func uploadFTP(ctx context.Context, cfg backendConfig, artifactPath string) (string, error) {
	if cfg.Host == "" {
		return "", errors.New("ftp backend requires host")
	}
	port := cfg.Port
	if port == 0 {
		port = 21
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(port))
	dialOpts := []ftp.DialOption{
		ftp.DialWithContext(ctx),
		ftp.DialWithTimeout(30 * time.Second),
	}
	if cfg.TLS || cfg.ExplicitTLS {
		tlsConfig := &tls.Config{
			ServerName:         cfg.Host,
			InsecureSkipVerify: cfg.InsecureSkipVerify,
		}
		if cfg.ExplicitTLS {
			dialOpts = append(dialOpts, ftp.DialWithExplicitTLS(tlsConfig))
		} else {
			dialOpts = append(dialOpts, ftp.DialWithTLS(tlsConfig))
		}
	}
	conn, err := ftp.Dial(addr, dialOpts...)
	if err != nil {
		return "", err
	}
	defer conn.Quit()
	username := cfg.Username
	if username == "" {
		username = "anonymous"
	}
	if err := conn.Login(username, cfg.Password); err != nil {
		return "", err
	}
	if cfg.RemoteDir != "" {
		if err := ensureFTPDir(conn, cfg.RemoteDir); err != nil {
			return "", err
		}
	}
	file, err := os.Open(artifactPath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	remotePath := pathpkg.Join(cfg.RemoteDir, filepath.Base(artifactPath))
	if err := conn.Stor(remotePath, file); err != nil {
		return "", err
	}
	return "ftp://" + pathpkg.Join(cfg.Host, remotePath), nil
}

func ensureFTPDir(conn *ftp.ServerConn, remoteDir string) error {
	remoteDir = strings.Trim(remoteDir, "/")
	if remoteDir == "" {
		return nil
	}
	current := ""
	for _, part := range strings.Split(remoteDir, "/") {
		current = pathpkg.Join(current, part)
		if err := conn.MakeDir(current); err != nil && !strings.Contains(strings.ToLower(err.Error()), "exist") {
			return err
		}
	}
	return nil
}

func objectKey(prefix, artifactPath string) string {
	name := filepath.Base(artifactPath)
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return name
	}
	return pathpkg.Join(prefix, name)
}
