// Copyright 2018 Google LLC. All Rights Reserved.
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

package loglist

import (
	"time"

	"github.com/golang/glog"
	"github.com/google/certificate-transparency-go/trillian/ctfe"
	"github.com/google/certificate-transparency-go/x509"
)

// LogRoots maps Log-URLs (stated at LogList) to the pools of their accepted
// root-certificates.
type LogRoots map[string]*ctfe.PEMCertPool

// ActiveLogs creates a new LogList containing only non-disqualified non-frozen
// logs from the original.
func (ll *LogList) ActiveLogs() LogList {
	var active LogList
	// Keep all the operators.
	active.Operators = ll.Operators
	for _, l := range ll.Logs {
		if (l.DisqualifiedAt <= 0 && l.FinalSTH == nil) || time.Until(time.Unix(int64(l.DisqualifiedAt), 0)) > 0 {
			active.Logs = append(active.Logs, l)
		}
	}
	return active
}

// Compatible creates a new LogList containing only the logs of original
// LogList that are compatible with the provided cert, according to
// the passed in collection of per-log roots. Logs that are missing from
// the collection are treated as always compatible and included, even if
// an empty cert root is passed in.
// Cert-root when provided is expected to be CA-cert.
func (ll *LogList) Compatible(cert *x509.Certificate, certRoot *x509.Certificate, roots LogRoots) LogList {
	var compatible LogList
	// Keep all the operators.
	compatible.Operators = ll.Operators

	// Check whether chain is ending with CA-cert.
	if certRoot != nil && !certRoot.IsCA {
		glog.Warningf("Compatible method expects fully rooted chain, while last cert of the chain provided is not root")
		return compatible
	}

	for _, l := range ll.Logs {
		// If root set is not defined, we treat Log as compatible assuming no
		// knowledge of its roots.
		if _, ok := roots[l.URL]; !ok {
			compatible.Logs = append(compatible.Logs, l)
			continue
		}
		if certRoot == nil {
			continue
		}

		// Check root is accepted.
		if roots[l.URL].Included(certRoot) {
			compatible.Logs = append(compatible.Logs, l)
		}
	}
	return compatible
}
