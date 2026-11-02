// Package jobs owns the closed pyxis-maintenance-jobs/v1 model and the
// repository-local .pyxis/maintenance/jobs.yaml store.
//
// Bind a Store with New. Create requires a SourceBinding only when it creates
// the file; subsequent reads never infer one. Read and List are byte-preserving.
// Update requires the replacement Job to retain the requested ID, and Delete
// removes only that ID. Decode and Encode are the shared parser boundary for
// consumers that already possess document bytes. Delivery is passive metadata:
// this package stores and validates it but performs no publication.
package jobs
