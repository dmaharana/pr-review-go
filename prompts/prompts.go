package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed *.tmpl
var promptFS embed.FS

var tmplCache = make(map[string]*template.Template)

func getTemplate(name string) (*template.Template, error) {
	if tmpl, ok := tmplCache[name]; ok {
		return tmpl, nil
	}
	content, err := promptFS.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("reading prompt template %q: %w", name, err)
	}
	tmpl, err := template.New(name).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parsing prompt template %q: %w", name, err)
	}
	tmplCache[name] = tmpl
	return tmpl, nil
}

func renderTemplate(name string, data any) (string, error) {
	tmpl, err := getTemplate(name)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template %q: %w", name, err)
	}
	return buf.String(), nil
}

// ReviewSystemData carries variables into review_system.tmpl.
type ReviewSystemData struct {
	ExtraInstructions string
}

// ReviewUserData carries variables into review_user.tmpl.
type ReviewUserData struct {
	Title       string
	Description string
	Author      string
	HeadBranch  string
	BaseBranch  string
	Diff        string
}

// RenderReviewSystem renders the review system prompt.
func RenderReviewSystem(data ReviewSystemData) (string, error) {
	return renderTemplate("review_system.tmpl", data)
}

// RenderReviewUser renders the review user prompt.
func RenderReviewUser(data ReviewUserData) (string, error) {
	return renderTemplate("review_user.tmpl", data)
}

// DescribeUserData carries variables into describe_user.tmpl.
type DescribeUserData struct {
	Title       string
	Description string
	HeadBranch  string
	BaseBranch  string
	Diff        string
}

// RenderDescribeSystem renders the describe system prompt.
func RenderDescribeSystem() (string, error) {
	return renderTemplate("describe_system.tmpl", nil)
}

// RenderDescribeUser renders the describe user prompt.
func RenderDescribeUser(data DescribeUserData) (string, error) {
	return renderTemplate("describe_user.tmpl", data)
}

// AskUserData carries variables into ask_user.tmpl.
type AskUserData struct {
	Question   string
	Title      string
	HeadBranch string
	BaseBranch string
	Diff       string
}

// RenderAskSystem renders the ask system prompt.
func RenderAskSystem() (string, error) {
	return renderTemplate("ask_system.tmpl", nil)
}

// RenderAskUser renders the ask user prompt.
func RenderAskUser(data AskUserData) (string, error) {
	return renderTemplate("ask_user.tmpl", data)
}
