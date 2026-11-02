package port

import "context"

// IssueReader exact-reads one item and its dependency facts without a frontier.
type IssueReader interface {
	ReadIssue(context.Context, int) (IssueSnapshot, error)
}
type IssueComment struct {
	ID   string `json:"id"`
	Body string `json:"body"`
}
type IssueDependency struct {
	Number int  `json:"number"`
	Closed bool `json:"closed"`
}
type IssueSnapshot struct {
	Number       int               `json:"number"`
	Title        string            `json:"title"`
	Body         string            `json:"body"`
	Open         bool              `json:"open"`
	Executable   bool              `json:"executable"`
	Comments     []IssueComment    `json:"comments"`
	Dependencies []IssueDependency `json:"dependencies"`
}
