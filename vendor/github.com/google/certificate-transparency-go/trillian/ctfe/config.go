// Copyright 2016 Google LLC. All Rights Reserved.
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

package ctfe

import (
	"crypto"
	"errors"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/golang/glog"
	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/trillian/ctfe/configpb"
	"github.com/google/certificate-transparency-go/x509"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

// ValidatedLogConfig represents the LogConfig with the information that has
// been successfully parsed as a result of validating it.
type ValidatedLogConfig struct {
	Config        *configpb.LogConfig
	PubKey        crypto.PublicKey
	PrivKey       proto.Message
	KeyUsages     []x509.ExtKeyUsage
	NotAfterStart *time.Time
	NotAfterLimit *time.Time
	FrozenSTH     *ct.SignedTreeHead
}

// LogConfigFromFile creates a slice of LogConfig options from the given
// filename, which should contain text or binary-encoded protobuf configuration
// data.
func LogConfigFromFile(filename string) ([]*configpb.LogConfig, error) {
	cfgBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var cfg configpb.LogConfigSet
	if txtErr := prototext.Unmarshal(cfgBytes, &cfg); txtErr != nil {
		if binErr := proto.Unmarshal(cfgBytes, &cfg); binErr != nil {
			return nil, fmt.Errorf("failed to parse LogConfigSet from %q as text protobuf (%v) or binary protobuf (%v)", filename, txtErr, binErr)
		}
	}

	if len(cfg.Config) == 0 {
		return nil, errors.New("empty log config found")
	}
	return cfg.Config, nil
}

// ToMultiLogConfig creates a multi backend config proto from the data
// loaded from a single-backend configuration file. All the log configs
// reference a default backend spec as provided.
func ToMultiLogConfig(cfg []*configpb.LogConfig, beSpec string) *configpb.LogMultiConfig {
	defaultBackend := &configpb.LogBackend{Name: "default", BackendSpec: beSpec}
	for _, c := range cfg {
		c.LogBackendName = defaultBackend.Name
	}
	return &configpb.LogMultiConfig{
		LogConfigs: &configpb.LogConfigSet{Config: cfg},
		Backends:   &configpb.LogBackendSet{Backend: []*configpb.LogBackend{defaultBackend}},
	}
}

// MultiLogConfigFromFile creates a LogMultiConfig proto from the given
// filename, which should contain text or binary-encoded protobuf configuration data.
// Does not do full validation of the config but checks that it is non empty.
func MultiLogConfigFromFile(filename string) (*configpb.LogMultiConfig, error) {
	cfgBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var cfg configpb.LogMultiConfig
	if txtErr := prototext.Unmarshal(cfgBytes, &cfg); txtErr != nil {
		if binErr := proto.Unmarshal(cfgBytes, &cfg); binErr != nil {
			return nil, fmt.Errorf("failed to parse LogMultiConfig from %q as text protobuf (%v) or binary protobuf (%v)", filename, txtErr, binErr)
		}
	}

	if len(cfg.LogConfigs.GetConfig()) == 0 || len(cfg.Backends.GetBackend()) == 0 {
		return nil, errors.New("config is missing backends and/or log configs")
	}
	return &cfg, nil
}

// ValidateLogConfig checks that a single log config is valid. In particular:
//  - A mirror log has a valid public key and no private key.
//  - A non-mirror log has a private, and optionally a public key (both valid).
//  - Each of NotBeforeStart and NotBeforeLimit, if set, is a valid timestamp
//    proto. If both are set then NotBeforeStart <= NotBeforeLimit.
//  - Merge delays (if present) are correct.
//  - Frozen STH (if present) is correct and signed by the provided public key.
// Returns the validated structures (useful to avoid double validation).
func ValidateLogConfig(cfg *configpb.LogConfig) (*ValidatedLogConfig, error) {
	if cfg.LogId == 0 {
		return nil, errors.New("empty log ID")
	}

	vCfg := ValidatedLogConfig{Config: cfg}

	// Validate the public key.
	if pubKey := cfg.PublicKey; pubKey != nil {
		var err error
		if vCfg.PubKey, err = x509.ParsePKIXPublicKey(pubKey.Der); err != nil {
			return nil, fmt.Errorf("x509.ParsePKIXPublicKey: %w", err)
		}
	} else if cfg.IsMirror {
		return nil, errors.New("empty public key for mirror")
	} else if cfg.FrozenSth != nil {
		return nil, errors.New("empty public key for frozen STH")
	}

	// Validate the private key.
	if !cfg.IsMirror {
		if cfg.PrivateKey == nil {
			return nil, errors.New("empty private key")
		}
		privKey, err := cfg.PrivateKey.UnmarshalNew()
		if err != nil {
			return nil, fmt.Errorf("invalid private key: %v", err)
		}
		vCfg.PrivKey = privKey
	} else if cfg.PrivateKey != nil {
		return nil, errors.New("unnecessary private key for mirror")
	}

	if cfg.RejectExpired && cfg.RejectUnexpired {
		return nil, errors.New("rejecting all certificates")
	}

	// Validate the extended key usages list.
	if len(cfg.ExtKeyUsages) > 0 {
		for _, kuStr := range cfg.ExtKeyUsages {
			if ku, ok := stringToKeyUsage[kuStr]; ok {
				// If "Any" is specified, then we can ignore the entire list and
				// just disable EKU checking.
				if ku == x509.ExtKeyUsageAny {
					glog.Infof("%s: Found ExtKeyUsageAny, allowing all EKUs", cfg.Prefix)
					vCfg.KeyUsages = nil
					break
				}
				vCfg.KeyUsages = append(vCfg.KeyUsages, ku)
			} else {
				return nil, fmt.Errorf("unknown extended key usage: %s", kuStr)
			}
		}
	}

	// Validate the time interval.
	start, limit := cfg.NotAfterStart, cfg.NotAfterLimit
	if start != nil {
		vCfg.NotAfterStart = &time.Time{}
		if err := start.CheckValid(); err != nil {
			return nil, fmt.Errorf("invalid start timestamp: %v", err)
		}
		*vCfg.NotAfterStart = start.AsTime()
	}
	if limit != nil {
		vCfg.NotAfterLimit = &time.Time{}
		if err := limit.CheckValid(); err != nil {
			return nil, fmt.Errorf("invalid limit timestamp: %v", err)
		}
		*vCfg.NotAfterLimit = limit.AsTime()
	}
	if start != nil && limit != nil && (*vCfg.NotAfterLimit).Before(*vCfg.NotAfterStart) {
		return nil, errors.New("limit before start")
	}

	switch {
	case cfg.MaxMergeDelaySec < 0:
		return nil, errors.New("negative maximum merge delay")
	case cfg.ExpectedMergeDelaySec < 0:
		return nil, errors.New("negative expected merge delay")
	case cfg.ExpectedMergeDelaySec > cfg.MaxMergeDelaySec:
		return nil, errors.New("expected merge delay exceeds MMD")
	}

	if sth := cfg.FrozenSth; sth != nil {
		verifier, err := ct.NewSignatureVerifier(vCfg.PubKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create signature verifier: %v", err)
		}
		if vCfg.FrozenSTH, err = (&ct.GetSTHResponse{
			TreeSize:          uint64(sth.TreeSize),
			Timestamp:         uint64(sth.Timestamp),
			SHA256RootHash:    sth.Sha256RootHash,
			TreeHeadSignature: sth.TreeHeadSignature,
		}).ToSignedTreeHead(); err != nil {
			return nil, fmt.Errorf("invalid frozen STH: %v", err)
		}
		if err := verifier.VerifySTHSignature(*vCfg.FrozenSTH); err != nil {
			return nil, fmt.Errorf("signature verification failed: %v", err)
		}
	}

	return &vCfg, nil
}

// LogBackendMap is a map from log backend names to LogBackend objects.
type LogBackendMap = map[string]*configpb.LogBackend

// BuildLogBackendMap returns a map from log backend names to the corresponding
// LogBackend objects. It returns an error unless all backends have unique
// non-empty names and specifications.
func BuildLogBackendMap(lbs *configpb.LogBackendSet) (LogBackendMap, error) {
	lbm := make(LogBackendMap)
	specs := make(map[string]bool)
	for _, be := range lbs.Backend {
		if len(be.Name) == 0 {
			return nil, fmt.Errorf("empty backend name: %v", be)
		}
		if len(be.BackendSpec) == 0 {
			return nil, fmt.Errorf("empty backend spec: %v", be)
		}
		if _, ok := lbm[be.Name]; ok {
			return nil, fmt.Errorf("duplicate backend name: %v", be)
		}
		if ok := specs[be.BackendSpec]; ok {
			return nil, fmt.Errorf("duplicate backend spec: %v", be)
		}
		lbm[be.Name] = be
		specs[be.BackendSpec] = true
	}
	return lbm, nil
}

func validateConfigs(cfg []*configpb.LogConfig) error {
	// Check that logs have no duplicate or empty prefixes. Apply other LogConfig
	// specific checks.
	logNameMap := make(map[string]bool)
	for _, logCfg := range cfg {
		if _, err := ValidateLogConfig(logCfg); err != nil {
			return fmt.Errorf("log config: %v: %v", err, logCfg)
		}
		if len(logCfg.Prefix) == 0 {
			return fmt.Errorf("log config: empty prefix: %v", logCfg)
		}
		if logNameMap[logCfg.Prefix] {
			return fmt.Errorf("log config: duplicate prefix: %s: %v", logCfg.Prefix, logCfg)
		}
		logNameMap[logCfg.Prefix] = true
	}

	return nil
}

// ValidateLogConfigs checks that a config is valid for use with a single log
// server. The rules applied are:
//
// 1. All log configs must be valid (see ValidateLogConfig).
// 2. The prefixes of configured logs must all be distinct and must not be
// empty.
// 3. The set of tree IDs must be distinct.
func ValidateLogConfigs(cfg []*configpb.LogConfig) error {
	if err := validateConfigs(cfg); err != nil {
		return err
	}

	// Check that logs have no duplicate tree IDs.
	treeIDs := make(map[int64]bool)
	for _, logCfg := range cfg {
		if treeIDs[logCfg.LogId] {
			return fmt.Errorf("log config: dup tree id: %d for: %v", logCfg.LogId, logCfg)
		}
		treeIDs[logCfg.LogId] = true
	}

	return nil
}

// ValidateLogMultiConfig checks that a config is valid for use with multiple
// backend log servers. The rules applied are the same as ValidateLogConfigs, as
// well as these additional rules:
//
// 1. The backend set must define a set of log backends with distinct
// (non empty) names and non empty backend specs.
// 2. The backend specs must all be distinct.
// 3. The log configs must all specify a log backend and each must be one of
// those defined in the backend set.
//
// Also, another difference is that the tree IDs need only to be distinct per
// backend.
//
// TODO(pavelkalinnikov): Replace the returned map with a fully fledged
// ValidatedLogMultiConfig that contains a ValidatedLogConfig for each log.
func ValidateLogMultiConfig(cfg *configpb.LogMultiConfig) (LogBackendMap, error) {
	backendMap, err := BuildLogBackendMap(cfg.Backends)
	if err != nil {
		return nil, err
	}

	if err := validateConfigs(cfg.GetLogConfigs().GetConfig()); err != nil {
		return nil, err
	}

	// Check that logs all reference a defined backend.
	logIDMap := make(map[string]bool)
	for _, logCfg := range cfg.LogConfigs.Config {
		if _, ok := backendMap[logCfg.LogBackendName]; !ok {
			return nil, fmt.Errorf("log config: references undefined backend: %s: %v", logCfg.LogBackendName, logCfg)
		}
		logIDKey := fmt.Sprintf("%s-%d", logCfg.LogBackendName, logCfg.LogId)
		if ok := logIDMap[logIDKey]; ok {
			return nil, fmt.Errorf("log config: dup tree id: %d for: %v", logCfg.LogId, logCfg)
		}
		logIDMap[logIDKey] = true
	}

	return backendMap, nil
}

var stringToKeyUsage = map[string]x509.ExtKeyUsage{
	"Any":                        x509.ExtKeyUsageAny,
	"ServerAuth":                 x509.ExtKeyUsageServerAuth,
	"ClientAuth":                 x509.ExtKeyUsageClientAuth,
	"CodeSigning":                x509.ExtKeyUsageCodeSigning,
	"EmailProtection":            x509.ExtKeyUsageEmailProtection,
	"IPSECEndSystem":             x509.ExtKeyUsageIPSECEndSystem,
	"IPSECTunnel":                x509.ExtKeyUsageIPSECTunnel,
	"IPSECUser":                  x509.ExtKeyUsageIPSECUser,
	"TimeStamping":               x509.ExtKeyUsageTimeStamping,
	"OCSPSigning":                x509.ExtKeyUsageOCSPSigning,
	"MicrosoftServerGatedCrypto": x509.ExtKeyUsageMicrosoftServerGatedCrypto,
	"NetscapeServerGatedCrypto":  x509.ExtKeyUsageNetscapeServerGatedCrypto,
}
