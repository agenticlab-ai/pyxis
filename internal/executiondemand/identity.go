// This file derives demand_id: the domain-separated identity of one
// execution-demand origin (design §3.3, DINV-5).
package executiondemand

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
)

// DemandIDDomain domain-separates demand identity from every other digest
// this package or a sibling design computes.
const DemandIDDomain = "execution-demand-id/v1"

// Identify derives o's demand_id: a digest over the domain tag, the origin
// schema/version and kind, the repository/frozen-plan/task identities, the
// source digest, the origin payload, policy identity, and final profile — each length-delimited so
// no field's bytes can be mistaken for a neighbor's (design §3.3).
//
// Identical canonical source and policy bytes reproduce one demand ID
// across retries and roles; mutating any identity-bearing field changes it.
// Tracker text, proposal text, timestamps, runtime attempts, and current
// policy aliases never enter this computation because Origin has no field
// to hold them in the first place.
func Identify(o Origin) (DemandID, error) {
	if err := o.validate(); err != nil {
		return "", err
	}
	repository, plan, task, source := o.coreIdentity()
	var payload, policyOrCompat, profile []byte
	switch o.Kind {
	case OriginPlanV2:
		p := *o.PlanV2
		payload = identityPlanV2PayloadBytes(p)
		policyOrCompat = identityPlanV2PolicyBytes(p)
		profile = identityProfileMinimumBytes(profileMinimumOf(p.Profile))

	}
	var buf bytes.Buffer
	for _, part := range [][]byte{
		[]byte(DemandIDDomain),
		[]byte(o.Schema),
		[]byte(o.Kind),
		[]byte(repository),
		identityPlanBytes(plan),
		[]byte(task),
		[]byte(source),
		payload,
		policyOrCompat,
		profile,
	} {
		writeLD(&buf, part)
	}
	sum := sha256.Sum256(buf.Bytes())
	// hex.EncodeToString always emits lowercase — matches the "lowercase
	// digest" the design requires with no further case-folding needed.
	return DemandID(hex.EncodeToString(sum[:])), nil
}
