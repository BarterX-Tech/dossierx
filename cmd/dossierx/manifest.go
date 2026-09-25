package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/manifest"
)

func newManifestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "manifest",
		Short: "Read the required per-module manifest.yaml: file, isolation view, neighbor catalog",
	}
	cmd.AddCommand(
		newManifestShowCmd(),
		newManifestListCmd(),
	)
	return commandGroup(cmd)
}

type manifestShowData struct {
	Module             string                      `json:"module"`
	Path               string                      `json:"path"`
	Summary            string                      `json:"summary,omitempty"`
	Provides           []string                    `json:"provides"`
	DependsOn          []string                    `json:"depends_on"`
	Findings           []manifestFindingData       `json:"findings"`
	ConstitutionDigest manifest.ConstitutionDigest `json:"constitution_digest"`
	Isolation          *manifest.IsolationView     `json:"isolation,omitempty"`
	Integration        *manifest.IntegrationView   `json:"integration,omitempty"`
}

type manifestFindingData struct {
	Lint    string `json:"lint"`
	Module  string `json:"module"`
	Message string `json:"message"`
}

type manifestListData struct {
	Count   int                     `json:"count"`
	Modules []manifest.CatalogEntry `json:"modules"`
}

func newManifestShowCmd() *cobra.Command {
	var isolation, integration, bodies bool
	cmd := &cobra.Command{
		Use:   "show <module>",
		Short: "Print one module's manifest.yaml; --isolation adds constitution digest + claim summaries, --integration adds neighbor blurbs",
		Args:  cobra.ExactArgs(1),
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			module := args[0]
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return cmdResult{}, err
			}
			if err := requireDeclaredModule(cfg, module, "manifest show"); err != nil {
				return cmdResult{}, err
			}
			view, err := manifest.Show(claims, cfg, module, isolation, integration, bodies)
			if err != nil {
				if manifest.IsIsolationOversize(err) {
					return cmdResult{}, cliout.Errorf(cliout.CodeViewTooLarge, "manifest show: %s", err.Error()).
						WithHint("omit --bodies or shorten claim text; isolation is summaries by default")
				}
				return cmdResult{}, err
			}
			data := manifestShowData{
				Module:             view.Module,
				Path:               view.Path,
				Summary:            view.Summary,
				Provides:           view.Provides,
				DependsOn:          view.DependsOn,
				Findings:           projectManifestFindings(view.Findings),
				ConstitutionDigest: view.ConstitutionDigest,
				Isolation:          view.Isolation,
				Integration:        view.Integration,
			}
			res := cmdResult{
				Data: data,
				Text: func() { writeManifestShowText(cmd, data, isolation, integration) },
			}
			if len(view.Findings) > 0 {
				return res, cliout.Errorf(cliout.CodeLintFailed,
					"manifest show: module-manifest: %d error-level finding(s)", len(view.Findings)).
					WithDetails(map[string]any{"lint_findings": data.Findings})
			}
			return res, nil
		}),
	}
	cmd.Flags().BoolVar(&isolation, "isolation", false, "emit constitution digest + this manifest + claim summaries (bodies opt-in via --bodies)")
	cmd.Flags().BoolVar(&integration, "integration", false, "add neighbor module catalog rows and depends_on membership edges")
	cmd.Flags().BoolVar(&bodies, "bodies", false, "include claim bodies in --isolation (refused if the view exceeds the isolation cap)")
	return cmd
}

func newManifestListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Catalog every configured module from its manifest.yaml blurb (summaries only)",
		Args:  cobra.NoArgs,
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return cmdResult{}, err
			}
			entries := manifest.List(claims, cfg)
			data := manifestListData{Count: len(entries), Modules: entries}
			return cmdResult{
				Data: data,
				Text: func() { writeManifestListText(cmd, data) },
			}, nil
		}),
	}
}

func requireDeclaredModule(cfg *config.Config, module, verb string) error {
	for _, m := range cfg.Modules {
		if m == module {
			return nil
		}
	}
	return cliout.Errorf(cliout.CodeUnknownModule, "%s: module %q is not declared in project.config.yaml", verb, module).
		WithHint("run: dossierx manifest list")
}

func projectManifestFindings(in []manifest.Finding) []manifestFindingData {
	out := make([]manifestFindingData, 0, len(in))
	for _, f := range in {
		out = append(out, manifestFindingData{
			Lint:    "module-manifest",
			Module:  f.Module,
			Message: f.Message,
		})
	}
	return out
}

func writeManifestShowText(cmd *cobra.Command, d manifestShowData, isolation, integration bool) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "manifest show: %s (%s)\n", d.Module, d.Path)
	fmt.Fprintf(out, "  constitution:      %s\n", d.ConstitutionDigest.Status)
	if d.Summary != "" {
		fmt.Fprintf(out, "  summary:           %s\n", d.Summary)
	}
	fmt.Fprintf(out, "  provides:          %s\n", joinOrNone(d.Provides))
	fmt.Fprintf(out, "  depends_on:        %s\n", joinOrNone(d.DependsOn))
	if len(d.Findings) > 0 {
		fmt.Fprintln(out, "  findings:")
		for _, f := range d.Findings {
			fmt.Fprintf(out, "    module-manifest: %s\n", f.Message)
		}
	}
	if isolation && d.Isolation != nil {
		fmt.Fprintf(out, "  isolation claims:  %d (draft from summaries; do not paste bodies)\n", len(d.Isolation.Claims))
		if len(d.Isolation.DraftHints.SuggestedProvides) > 0 {
			fmt.Fprintf(out, "  suggested provides: %s\n", strings.Join(d.Isolation.DraftHints.SuggestedProvides, ", "))
		}
	}
	if integration && d.Integration != nil {
		fmt.Fprintf(out, "  neighbors:         %d\n", len(d.Integration.Neighbors))
		for _, n := range d.Integration.Neighbors {
			fmt.Fprintf(out, "    %s via %s — %s\n", n.Module, n.Via, n.Summary)
		}
	}
}

func writeManifestListText(cmd *cobra.Command, d manifestListData) {
	out := cmd.OutOrStdout()
	if d.Count == 0 {
		fmt.Fprintln(out, "manifest list: this project declares no modules")
		return
	}
	for _, e := range d.Modules {
		state := "in progress"
		if e.Locked {
			state = "locked"
		}
		fmt.Fprintf(out, "%s %s (%d/%d claims locked, %d findings) — %s\n",
			e.Module, state, e.LockedClaims, e.ClaimCount, e.Findings, e.Summary)
	}
	fmt.Fprintf(out, "manifest list: %d module(s)\n", d.Count)
}

func joinOrNone(ids []string) string {
	if len(ids) == 0 {
		return "(none)"
	}
	return strings.Join(ids, ", ")
}
