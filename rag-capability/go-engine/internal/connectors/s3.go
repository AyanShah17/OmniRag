package connectors

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type S3Connector struct {
	name            string
	bucket          string
	region          string
	prefix          string
	accessKeyID     string
	secretAccessKey string
	endpointURL     string
	client          *http.Client
}

func NewS3Connector(name string, config map[string]interface{}) (*S3Connector, error) {
	bucket, _ := config["bucket"].(string)
	if bucket == "" {
		bucket, _ = config["bucket_name"].(string)
	}
	if bucket == "" {
		return nil, fmt.Errorf("s3 connector requires 'bucket'")
	}

	region, _ := config["region"].(string)
	if region == "" {
		region = "us-east-1"
	}

	prefix, _ := config["prefix"].(string)
	accessKey, _ := config["access_key_id"].(string)
	secretKey, _ := config["secret_access_key"].(string)
	endpointURL, _ := config["endpoint_url"].(string)

	return &S3Connector{
		name:            name,
		bucket:          bucket,
		region:          region,
		prefix:          prefix,
		accessKeyID:     accessKey,
		secretAccessKey: secretKey,
		endpointURL:     endpointURL,
		client:          &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (s *S3Connector) Type() string { return "s3" }
func (s *S3Connector) Name() string { return s.name }

func (s *S3Connector) ValidateConfig(ctx context.Context) error {
	if s.bucket == "" {
		return fmt.Errorf("s3 bucket is required")
	}
	return nil
}

type listBucketResult struct {
	XMLName  xml.Name `xml:"ListBucketResult"`
	Name     string   `xml:"Name"`
	Contents []struct {
		Key          string    `xml:"Key"`
		LastModified time.Time `xml:"LastModified"`
		ETag         string    `xml:"ETag"`
		Size         int64     `xml:"Size"`
		StorageClass string    `xml:"StorageClass"`
	} `xml:"Contents"`
}

func (s *S3Connector) ListObjects(ctx context.Context, prefix string) ([]*StorageObject, error) {
	effectivePrefix := s.prefix
	if prefix != "" {
		effectivePrefix = path.Join(effectivePrefix, prefix)
	}

	var endpoint string
	if s.endpointURL != "" {
		endpoint = fmt.Sprintf("%s/%s", strings.TrimRight(s.endpointURL, "/"), s.bucket)
	} else {
		endpoint = fmt.Sprintf("https://%s.s3.%s.amazonaws.com", s.bucket, s.region)
	}

	reqURL := fmt.Sprintf("%s?list-type=2", endpoint)
	if effectivePrefix != "" {
		reqURL += fmt.Sprintf("&prefix=%s", url.QueryEscape(effectivePrefix))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	// Sign request if credentials provided
	if s.accessKeyID != "" && s.secretAccessKey != "" {
		s.signAWSv4(req, []byte{})
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list s3 objects: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("s3 list objects returned status %d: %s", resp.StatusCode, string(body))
	}

	var result listBucketResult
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse s3 response xml: %w", err)
	}

	var objects []*StorageObject
	for _, item := range result.Contents {
		// Skip folder markers
		if strings.HasSuffix(item.Key, "/") {
			continue
		}
		objects = append(objects, &StorageObject{
			Key:          item.Key,
			ETag:         strings.Trim(item.ETag, "\""),
			Size:         item.Size,
			LastModified: item.LastModified,
			ContentType:  detectContentType(item.Key),
			Metadata: map[string]interface{}{
				"source":        "s3",
				"bucket":        s.bucket,
				"storage_class": item.StorageClass,
			},
		})
	}

	return objects, nil
}

func (s *S3Connector) FetchObject(ctx context.Context, key string) ([]byte, *StorageObject, error) {
	var endpoint string
	if s.endpointURL != "" {
		endpoint = fmt.Sprintf("%s/%s/%s", strings.TrimRight(s.endpointURL, "/"), s.bucket, key)
	} else {
		endpoint = fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, key)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, nil, err
	}

	if s.accessKeyID != "" && s.secretAccessKey != "" {
		s.signAWSv4(req, []byte{})
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch s3 object: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("s3 get object returned status %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read s3 object body: %w", err)
	}

	etag := strings.Trim(resp.Header.Get("ETag"), "\"")
	lastMod, _ := http.ParseTime(resp.Header.Get("Last-Modified"))

	obj := &StorageObject{
		Key:          key,
		ETag:         etag,
		Size:         int64(len(data)),
		LastModified: lastMod,
		ContentType:  resp.Header.Get("Content-Type"),
		Metadata: map[string]interface{}{
			"source": "s3",
			"bucket": s.bucket,
		},
	}

	return data, obj, nil
}

// AWS v4 simple signer implementation
func (s *S3Connector) signAWSv4(req *http.Request, payload []byte) {
	t := time.Now().UTC()
	amzDate := t.Format("20060102T150405Z")
	dateStamp := t.Format("20060102")

	req.Header.Set("x-amz-date", amzDate)
	payloadHash := sha256.Sum256(payload)
	payloadHashHex := hex.EncodeToString(payloadHash[:])
	req.Header.Set("x-amz-content-sha256", payloadHashHex)

	canonicalURI := req.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalQuery := req.URL.RawQuery

	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n", req.Host, payloadHashHex, amzDate)
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	canonicalReq := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s", req.Method, canonicalURI, canonicalQuery, canonicalHeaders, signedHeaders, payloadHashHex)
	canonicalReqHash := sha256.Sum256([]byte(canonicalReq))

	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, s.region)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s", amzDate, credentialScope, hex.EncodeToString(canonicalReqHash[:]))

	kDate := hmacSHA256([]byte("AWS4"+s.secretAccessKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(s.region))
	kService := hmacSHA256(kRegion, []byte("s3"))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	signature := hex.EncodeToString(hmacSHA256(kSigning, []byte(stringToSign)))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", s.accessKeyID, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)
}

func hmacSHA256(key []byte, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func detectContentType(filename string) string {
	ext := strings.ToLower(path.Ext(filename))
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".md", ".markdown":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	case ".csv":
		return "text/csv"
	case ".html", ".htm":
		return "text/html"
	case ".json":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

// Avoid unused import warning
var _ = bytes.Buffer{}
