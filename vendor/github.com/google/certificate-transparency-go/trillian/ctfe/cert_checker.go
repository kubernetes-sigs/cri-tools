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
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/google/certificate-transparency-go/asn1"
	"github.com/google/certificate-transparency-go/x509"
)

// IsPrecertificate tests if a certificate is a pre-certificate as defined in CT.
// An error is returned if the CT extension is present but is not ASN.1 NULL as defined
// by the spec.
func IsPrecertificate(cert *x509.Certificate) (bool, error) {
	for _, ext := range cert.Extensions {
		if x509.OIDExtensionCTPoison.Equal(ext.Id) {
			if !ext.Critical || !bytes.Equal(asn1.NullBytes, ext.Value) {
				return false, fmt.Errorf("CT poison ext is not critical or invalid: %v", ext)
			}

			return true, nil
		}
	}

	return false, nil
}

// ValidateChain takes the certificate chain as it was parsed from a JSON request. Ensures all
// elements in the chain decode as X.509 certificates. Ensures that there is a valid path from the
// end entity certificate in the chain to a trusted root cert, possibly using the intermediates
// supplied in the chain. Then applies the RFC requirement that the path must involve all
// the submitted chain in the order of submission.
func ValidateChain(rawChain [][]byte, validationOpts CertValidationOpts) ([]*x509.Certificate, error) {
	// First make sure the certs parse as X.509
	chain := make([]*x509.Certificate, 0, len(rawChain))
	intermediatePool := NewPEMCertPool()

	for i, certBytes := range rawChain {
		cert, err := x509.ParseCertificate(certBytes)
		if x509.IsFatal(err) {
			return nil, err
		}

		chain = append(chain, cert)

		// All but the first cert form part of the intermediate pool
		if i > 0 {
			intermediatePool.AddCert(cert)
		}
	}

	naStart := validationOpts.notAfterStart
	naLimit := validationOpts.notAfterLimit
	cert := chain[0]

	// Check whether the expiry date of the cert is within the acceptable range.
	if naStart != nil && cert.NotAfter.Before(*naStart) {
		return nil, fmt.Errorf("certificate NotAfter (%v) < %v", cert.NotAfter, *naStart)
	}
	if naLimit != nil && !cert.NotAfter.Before(*naLimit) {
		return nil, fmt.Errorf("certificate NotAfter (%v) >= %v", cert.NotAfter, *naLimit)
	}

	if validationOpts.acceptOnlyCA && !cert.IsCA {
		return nil, errors.New("only certificates with CA bit set are accepted")
	}

	now := validationOpts.currentTime
	if now.IsZero() {
		now = time.Now()
	}
	expired := now.After(cert.NotAfter)
	if validationOpts.rejectExpired && expired {
		return nil, errors.New("rejecting expired certificate")
	}
	if validationOpts.rejectUnexpired && !expired {
		return nil, errors.New("rejecting unexpired certificate")
	}

	// Check for unwanted extension types, if required.
	// TODO(al): Refactor CertValidationOpts c'tor to a builder pattern and
	// pre-calc this in there
	if len(validationOpts.rejectExtIds) != 0 {
		badIDs := make(map[string]bool)
		for _, id := range validationOpts.rejectExtIds {
			badIDs[id.String()] = true
		}
		for idx, ext := range cert.Extensions {
			extOid := ext.Id.String()
			if _, ok := badIDs[extOid]; ok {
				return nil, fmt.Errorf("rejecting certificate containing extension %v at index %d", extOid, idx)
			}
		}
	}

	// TODO(al): Refactor CertValidationOpts c'tor to a builder pattern and
	// pre-calc this in there too.
	if len(validationOpts.extKeyUsages) > 0 {
		acceptEKUs := make(map[x509.ExtKeyUsage]bool)
		for _, eku := range validationOpts.extKeyUsages {
			acceptEKUs[eku] = true
		}
		good := false
		for _, certEKU := range cert.ExtKeyUsage {
			if _, ok := acceptEKUs[certEKU]; ok {
				good = true
				break
			}
		}
		if !good {
			return nil, fmt.Errorf("rejecting certificate without EKU in %v", validationOpts.extKeyUsages)
		}
	}

	// We can now do the verification.  Use fairly lax options for verification, as
	// CT is intended to observe certificates rather than police them.
	verifyOpts := x509.VerifyOptions{
		Roots:             validationOpts.trustedRoots.CertPool(),
		CurrentTime:       now,
		Intermediates:     intermediatePool.CertPool(),
		DisableTimeChecks: true,
		// Precertificates have the poison extension; also the Go library code does not
		// support the standard PolicyConstraints extension (which is required to be marked
		// critical, RFC 5280 s4.2.1.11), so never check unhandled critical extensions.
		DisableCriticalExtensionChecks: true,
		// Pre-issued precertificates have the Certificate Transparency EKU; also some
		// leaves have unknown EKUs that should not be bounced just because the intermediate
		// does not also have them (cf. https://github.com/golang/go/issues/24590) so
		// disable EKU checks inside the x509 library, but we've already done our own check
		// on the leaf above.
		DisableEKUChecks: true,
		// Path length checks get confused by the presence of an additional
		// pre-issuer intermediate, so disable them.
		DisablePathLenChecks:        true,
		DisableNameConstraintChecks: true,
		DisableNameChecks:           false,
		KeyUsages:                   validationOpts.extKeyUsages,
	}

	verifiedChains, err := cert.Verify(verifyOpts)
	if err != nil {
		return nil, err
	}

	if len(verifiedChains) == 0 {
		return nil, errors.New("no path to root found when trying to validate chains")
	}

	// Verify might have found multiple paths to roots. Now we check that we have a path that
	// uses all the certs in the order they were submitted so as to comply with RFC 6962
	// requirements detailed in Section 3.1.
	for _, verifiedChain := range verifiedChains {
		if chainsEquivalent(chain, verifiedChain) {
			return verifiedChain, nil
		}
	}

	return nil, errors.New("no RFC compliant path to root found when trying to validate chain")
}

func chainsEquivalent(inChain []*x509.Certificate, verifiedChain []*x509.Certificate) bool {
	// The verified chain includes a root, but the input chain may or may not include a
	// root (RFC 6962 s4.1/ s4.2 "the last [certificate] is either the root certificate
	// or a certificate that chains to a known root certificate").
	if len(inChain) != len(verifiedChain) && len(inChain) != (len(verifiedChain)-1) {
		return false
	}

	for i, certInChain := range inChain {
		if !certInChain.Equal(verifiedChain[i]) {
			return false
		}
	}
	return true
}
