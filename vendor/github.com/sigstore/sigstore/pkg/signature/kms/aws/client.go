//
// Copyright 2021 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aws

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/ReneKroon/ttlcache/v2"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/pkg/errors"
	"github.com/sigstore/sigstore/pkg/signature"
	sigkms "github.com/sigstore/sigstore/pkg/signature/kms"
)

func init() {
	sigkms.AddProvider(ReferenceScheme, func(_ context.Context, keyResourceID string, _ crypto.Hash, _ ...signature.RPCOption) (sigkms.SignerVerifier, error) {
		return LoadSignerVerifier(keyResourceID)
	})
}

const (
	cacheKey = "signer"
	// ReferenceScheme schemes for various KMS services are copied from https://github.com/google/go-cloud/tree/master/secrets
	ReferenceScheme = "awskms://"
)

type awsClient struct {
	client   *kms.KMS
	endpoint string
	keyID    string
	alias    string
	keyCache *ttlcache.Cache
}

var (
	errKMSReference = errors.New("kms specification should be in the format awskms://[ENDPOINT]/[ID/ALIAS/ARN] (endpoint optional)")

	// Key ID/ALIAS/ARN conforms to KMS standard documented here: https://docs.aws.amazon.com/kms/latest/developerguide/concepts.html#key-id
	// Key format examples:
	// Key ID: awskms:///1234abcd-12ab-34cd-56ef-1234567890ab
	// Key ID with endpoint: awskms://localhost:4566/1234abcd-12ab-34cd-56ef-1234567890ab
	// Key ARN: awskms:///arn:aws:kms:us-east-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab
	// Key ARN with endpoint: awskms://localhost:4566/arn:aws:kms:us-east-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab
	// Alias name: awskms:///alias/ExampleAlias
	// Alias name with endpoint: awskms://localhost:4566/alias/ExampleAlias
	// Alias ARN: awskms:///arn:aws:kms:us-east-2:111122223333:alias/ExampleAlias
	// Alias ARN with endpoint: awskms://localhost:4566/arn:aws:kms:us-east-2:111122223333:alias/ExampleAlias
	uuidRE      = `m?r?k?-?[A-Fa-f0-9]{8}-?[A-Fa-f0-9]{4}-?[A-Fa-f0-9]{4}-?[A-Fa-f0-9]{4}-?[A-Fa-f0-9]{12}`
	arnRE       = `arn:(?:aws|aws-us-gov):kms:[a-z0-9-]+:\d{12}:`
	hostRE      = `([^/]*)/`
	keyIDRE     = regexp.MustCompile(`^awskms://` + hostRE + `(` + uuidRE + `)$`)
	keyARNRE    = regexp.MustCompile(`^awskms://` + hostRE + `(` + arnRE + `key/` + uuidRE + `)$`)
	aliasNameRE = regexp.MustCompile(`^awskms://` + hostRE + `((alias/.*))$`)
	aliasARNRE  = regexp.MustCompile(`^awskms://` + hostRE + `(` + arnRE + `(alias/.*))$`)
	allREs      = []*regexp.Regexp{keyIDRE, keyARNRE, aliasNameRE, aliasARNRE}
)

// ValidReference returns a non-nil error if the reference string is invalid
func ValidReference(ref string) error {
	for _, re := range allREs {
		if re.MatchString(ref) {
			return nil
		}
	}
	return errKMSReference
}

func parseReference(resourceID string) (endpoint, keyID, alias string, err error) {
	var v []string
	for _, re := range allREs {
		v = re.FindStringSubmatch(resourceID)
		if len(v) >= 3 {
			endpoint, keyID = v[1], v[2]
			if len(v) == 4 {
				alias = v[3]
			}
			return
		}
	}
	err = errors.Errorf("invalid awskms format %q", resourceID)
	return
}

func newAWSClient(keyResourceID string) (a *awsClient, err error) {
	a = &awsClient{}
	a.endpoint, a.keyID, a.alias, err = parseReference(keyResourceID)
	if err != nil {
		return nil, err
	}

	err = a.setupClient()
	if err != nil {
		return nil, err
	}

	a.keyCache = ttlcache.NewCache()
	a.keyCache.SetLoaderFunction(a.keyCacheLoaderFunction)
	a.keyCache.SkipTTLExtensionOnHit(true)
	return
}

func (a *awsClient) setupClient() (err error) {
	var sess *session.Session
	config := &aws.Config{}
	if a.endpoint != "" {
		config.Endpoint = aws.String("https://" + a.endpoint)
	}
	if os.Getenv("AWS_TLS_INSECURE_SKIP_VERIFY") == "1" {
		config.HTTPClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}} // nolint: gosec
	}
	sess, err = session.NewSession(config)
	if err != nil {
		return errors.Wrap(err, "new aws session")
	}
	a.client = kms.New(sess)
	return
}

type cmk struct {
	KeyMetadata *kms.KeyMetadata
	PublicKey   crypto.PublicKey
}

func (c *cmk) HashFunc() crypto.Hash {
	switch *c.KeyMetadata.SigningAlgorithms[0] {
	case kms.SigningAlgorithmSpecRsassaPssSha256, kms.SigningAlgorithmSpecRsassaPkcs1V15Sha256, kms.SigningAlgorithmSpecEcdsaSha256:
		return crypto.SHA256
	case kms.SigningAlgorithmSpecRsassaPssSha384, kms.SigningAlgorithmSpecRsassaPkcs1V15Sha384, kms.SigningAlgorithmSpecEcdsaSha384:
		return crypto.SHA384
	case kms.SigningAlgorithmSpecRsassaPssSha512, kms.SigningAlgorithmSpecRsassaPkcs1V15Sha512, kms.SigningAlgorithmSpecEcdsaSha512:
		return crypto.SHA512
	default:
		return 0
	}
}

func (c *cmk) Verifier() (signature.Verifier, error) {
	switch *c.KeyMetadata.SigningAlgorithms[0] {
	case kms.SigningAlgorithmSpecRsassaPssSha256, kms.SigningAlgorithmSpecRsassaPssSha384, kms.SigningAlgorithmSpecRsassaPssSha512:
		return signature.LoadRSAPSSVerifier(c.PublicKey.(*rsa.PublicKey), c.HashFunc(), nil)
	case kms.SigningAlgorithmSpecRsassaPkcs1V15Sha256, kms.SigningAlgorithmSpecRsassaPkcs1V15Sha384, kms.SigningAlgorithmSpecRsassaPkcs1V15Sha512:
		return signature.LoadRSAPKCS1v15Verifier(c.PublicKey.(*rsa.PublicKey), c.HashFunc())
	case kms.SigningAlgorithmSpecEcdsaSha256, kms.SigningAlgorithmSpecEcdsaSha384, kms.SigningAlgorithmSpecEcdsaSha512:
		return signature.LoadECDSAVerifier(c.PublicKey.(*ecdsa.PublicKey), c.HashFunc())
	default:
		return nil, fmt.Errorf("signing algorithm unsupported")
	}
}

func (a *awsClient) keyCacheLoaderFunction(key string) (cmk interface{}, ttl time.Duration, err error) {
	return a.keyCacheLoaderFunctionWithContext(context.Background())(key)
}

func (a *awsClient) keyCacheLoaderFunctionWithContext(ctx context.Context) ttlcache.LoaderFunction {
	return func(key string) (cmk interface{}, ttl time.Duration, err error) {
		cmk, err = a.fetchCMK(ctx)
		ttl = time.Second * 300
		return
	}
}

func (a *awsClient) fetchCMK(ctx context.Context) (*cmk, error) {
	var err error
	cmk := &cmk{}
	cmk.PublicKey, err = a.fetchPublicKey(ctx)
	if err != nil {
		return nil, err
	}
	cmk.KeyMetadata, err = a.fetchKeyMetadata(ctx)
	if err != nil {
		return nil, err
	}
	return cmk, nil
}

func (a *awsClient) getHashFunc(ctx context.Context) (crypto.Hash, error) {
	cmk, err := a.getCMK(ctx)
	if err != nil {
		return 0, err
	}
	return cmk.HashFunc(), nil
}

func (a *awsClient) getCMK(ctx context.Context) (*cmk, error) {
	c, err := a.keyCache.GetByLoader(cacheKey, a.keyCacheLoaderFunctionWithContext(ctx))
	if err != nil {
		return nil, err
	}

	return c.(*cmk), nil
}

func (a *awsClient) createKey(ctx context.Context, algorithm string) (crypto.PublicKey, error) {
	if a.alias == "" {
		return nil, errors.New("must use alias key format")
	}

	// look for existing key first
	out, err := a.public(ctx)
	if err == nil {
		return out, nil
	}

	// return error if not *kms.NotFoundException
	var errNotFound *kms.NotFoundException
	if !errors.As(err, &errNotFound) {
		return nil, errors.Wrap(err, "looking up key")
	}

	usage := kms.KeyUsageTypeSignVerify
	description := "Created by Sigstore"
	key, err := a.client.CreateKeyWithContext(ctx, &kms.CreateKeyInput{
		CustomerMasterKeySpec: &algorithm,
		KeyUsage:              &usage,
		Description:           &description,
	})
	if err != nil {
		return nil, errors.Wrap(err, "creating key")
	}

	_, err = a.client.CreateAliasWithContext(ctx, &kms.CreateAliasInput{
		AliasName:   &a.alias,
		TargetKeyId: key.KeyMetadata.KeyId,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "creating alias %q", a.alias)
	}

	return a.public(ctx)
}

func (a *awsClient) verify(ctx context.Context, sig, message io.Reader, opts ...signature.VerifyOption) error {
	cmk, err := a.getCMK(ctx)
	if err != nil {
		return err
	}
	verifier, err := cmk.Verifier()
	if err != nil {
		return err
	}
	return verifier.VerifySignature(sig, message, opts...)
}

func (a *awsClient) verifyRemotely(ctx context.Context, sig []byte, digest []byte) error {
	cmk, err := a.getCMK(ctx)
	if err != nil {
		return err
	}
	alg := cmk.KeyMetadata.SigningAlgorithms[0]
	messageType := kms.MessageTypeDigest
	_, err = a.client.VerifyWithContext(ctx, &kms.VerifyInput{
		KeyId:            &a.keyID,
		Message:          digest,
		MessageType:      &messageType,
		Signature:        sig,
		SigningAlgorithm: alg,
	})
	return errors.Wrap(err, "unable to verify signature")
}

func (a *awsClient) public(ctx context.Context) (crypto.PublicKey, error) {
	key, err := a.keyCache.GetByLoader(cacheKey, a.keyCacheLoaderFunctionWithContext(ctx))
	if err != nil {
		return nil, err
	}
	return key.(*cmk).PublicKey, nil
}

func (a *awsClient) sign(ctx context.Context, digest []byte, _ crypto.Hash) ([]byte, error) {
	cmk, err := a.getCMK(ctx)
	if err != nil {
		return nil, err
	}
	alg := cmk.KeyMetadata.SigningAlgorithms[0]

	messageType := kms.MessageTypeDigest
	out, err := a.client.SignWithContext(ctx, &kms.SignInput{
		KeyId:            &a.keyID,
		Message:          digest,
		MessageType:      &messageType,
		SigningAlgorithm: alg,
	})
	if err != nil {
		return nil, errors.Wrap(err, "signing with kms")
	}
	return out.Signature, nil
}

func (a *awsClient) fetchPublicKey(ctx context.Context) (crypto.PublicKey, error) {
	out, err := a.client.GetPublicKeyWithContext(ctx, &kms.GetPublicKeyInput{
		KeyId: &a.keyID,
	})
	if err != nil {
		return nil, errors.Wrap(err, "getting public key")
	}
	key, err := x509.ParsePKIXPublicKey(out.PublicKey)
	if err != nil {
		return nil, errors.Wrap(err, "parsing public key")
	}
	return key, nil
}

func (a *awsClient) fetchKeyMetadata(ctx context.Context) (*kms.KeyMetadata, error) {
	out, err := a.client.DescribeKeyWithContext(ctx, &kms.DescribeKeyInput{
		KeyId: &a.keyID,
	})
	if err != nil {
		return nil, errors.Wrap(err, "getting key metadata")
	}
	return out.KeyMetadata, nil
}
