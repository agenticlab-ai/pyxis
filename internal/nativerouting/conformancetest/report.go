package conformancetest

import (
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

func reportRow(cell Cell, selected nativerouting.SelectedProfile) QualificationRow {
	return QualificationRow{
		CellID: cell.CellID, Harness: cell.Harness, Role: cell.Role, SemanticProfileID: cell.SemanticProfileID,
		NativeProfileID: selected.NativeProfileID, QualificationID: selected.QualificationID,
		QualificationDigest: selected.QualificationDigest, AdapterGeneration: selected.AdapterGeneration,
	}
}

func qualificationEvidence(selected nativerouting.SelectedProfile, qualification nativerouting.Qualification) QualificationEvidenceRecord {
	claims := make([]QualificationEvidence, 0, len(qualification.Claims))
	for _, claim := range qualification.Claims {
		claims = append(claims, QualificationEvidence{
			Component: claim.Component, AssertedLevel: claim.AssertedLevel, Method: claim.Method,
			FixtureDigest: claim.FixtureDigest, ResultDigest: claim.ResultDigest,
			Threshold: claim.Threshold, Result: claim.Result, EvidenceReference: claim.EvidenceReference,
			EvaluatorVersion: claim.EvaluatorVersion, ResponsibleEvaluator: claim.ResponsibleEvaluator,
		})
	}
	sort.Slice(claims, func(i, j int) bool { return claims[i].Component < claims[j].Component })
	validity := qualification.Validity
	validity.Invalidations = append([]nativerouting.Invalidation(nil), validity.Invalidations...)
	return QualificationEvidenceRecord{
		Harness: selected.Harness, Role: selected.Role,
		NativeProfileID: selected.NativeProfileID, QualificationID: selected.QualificationID,
		QualificationDigest: selected.QualificationDigest, AdapterGeneration: selected.AdapterGeneration,
		Validity: validity, Claims: claims,
	}
}
