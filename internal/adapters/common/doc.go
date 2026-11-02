// Package common holds shared medium support that is not a port implementation.
//
// Port adapters live under adapters/<port>/<medium>. Support more than one
// GitHub port needs — credential scoping, actor identity — lives under
// common/github so those adapters do not each grow a private copy (AP-11).
// Shared Forgejo HTTP transport lives under common/forgejo for the same
// reason: several ports share one authenticated client. Engine packages may
// import common; they still must not import a port adapter.
package common
