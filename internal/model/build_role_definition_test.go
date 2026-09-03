package model

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestBuildRoleDefinitionMatchesDocComments holds buildRoleDefinitions to the
// BuildRole* const doc comments in claim.go, under the one trim rule the map's
// comment states: drop the leading identifier and, when the next word is "is",
// that word too; join the comment lines with single spaces; trim nothing at
// the end. The viewer headers, the CLI's %% lines and the payload all print
// BuildRoleDefinition, so a comment edited without the map (or the reverse)
// would otherwise ship superseded wording with go test green.
func TestBuildRoleDefinitionMatchesDocComments(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "claim.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse claim.go: %v", err)
	}

	found := map[string]string{} // const name -> trimmed doc comment
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || !strings.HasPrefix(vs.Names[0].Name, "BuildRole") {
				continue
			}
			if vs.Doc == nil {
				t.Errorf("%s has no doc comment to hold its definition to", vs.Names[0].Name)
				continue
			}
			found[vs.Names[0].Name] = trimDefinition(vs.Names[0].Name, vs.Doc.Text())
		}
	}
	if len(found) < 6 {
		t.Fatalf("found %d BuildRole* consts with doc comments in claim.go, want at least 6: %v", len(found), found)
	}

	consts := map[string]BuildRole{
		"BuildRoleOrientation":  BuildRoleOrientation,
		"BuildRoleSchema":       BuildRoleSchema,
		"BuildRoleBehavior":     BuildRoleBehavior,
		"BuildRoleAPI":          BuildRoleAPI,
		"BuildRoleVerification": BuildRoleVerification,
		"BuildRoleOutOfScope":   BuildRoleOutOfScope,
	}
	if len(buildRoleDefinitions) != len(consts) {
		t.Errorf("buildRoleDefinitions has %d entries, want %d", len(buildRoleDefinitions), len(consts))
	}
	for name, role := range consts {
		want, ok := found[name]
		if !ok {
			t.Errorf("claim.go declares no const %s", name)
			continue
		}
		if got := BuildRoleDefinition(role); got != want {
			t.Errorf("BuildRoleDefinition(%s) disagrees with the doc comment\n  map:     %q\n  comment: %q", name, got, want)
		}
	}
	if got := BuildRoleDefinition(BuildRole("nope")); got != "" {
		t.Errorf("BuildRoleDefinition of an unknown role = %q, want \"\"", got)
	}
}

// trimDefinition applies the rule the map's doc comment states to one
// const's doc text: whitespace-normalised, leading identifier dropped, and a
// following "is" dropped with it.
func trimDefinition(name, doc string) string {
	words := strings.Fields(doc)
	if len(words) > 0 && words[0] == name {
		words = words[1:]
		if len(words) > 0 && words[0] == "is" {
			words = words[1:]
		}
	}
	return strings.Join(words, " ")
}
