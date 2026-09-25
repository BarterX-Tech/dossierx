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
	IsolationBudget    *manifest.IsolationBudget   `json:"isolation_budget,omitempty"`
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
	var isolation, integration bool
	cmd := &cobra.Command{
		Use:   "show <module>",
		Short: "Print one module's manifest.yaml; --isolation adds the constitution, project claims index and claim summaries, --integration adds neighbors and what they provide",
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
			view, err := manifest.Show(claims, cfg, module, manifest.ShowOptions{
				Isolation:         isolation,
				Integration:       integration,
				ConstitutionState: string(constitutionVerdict(cfg).State),
			})
			if err != nil {
				if manifest.IsIsolationOversize(err) {
					return cmdResult{}, cliout.Errorf(cliout.CodeViewTooLarge, "manifest show: %s", err.Error()).
						WithDetails(manifest.IsolationOversizeDetails(err)).
						WithHint("trim this module's claim summaries or manifest, or split the module; error.details has module_bytes against module_budget. " +
							"The module budget counts bytes, so multibyte (e.g. CJK) summaries overflow it long before max_claim_summary_chars; " +
							"raising max_claims_per_module or max_claim_summary_chars only makes room for more, and is the human's call, never a fix")
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
				IsolationBudget:    view.IsolationBudget,
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
	cmd.Flags().BoolVar(&isolation, "isolation", false, "emit constitution text + project claims index + this manifest + claim summaries (no bodies; read one with claim show <id>)")
	cmd.Flags().BoolVar(&integration, "integration", false, "add each depended-on module's summary, provides and provided-claim summaries, depends_on edges and the project claims index")
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
	fmt.Fprintf(out, "  constitution:      %s\n", constitutionLine(d.ConstitutionDigest))
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
		fmt.Fprintf(out, "  project claims:    %d\n", len(d.Isolation.Shared.ProjectClaims))
		for _, e := range d.Isolation.Shared.ProjectClaims {
			fmt.Fprintf(out, "    %s — %s\n", e.ID, e.Summary)
		}
		fmt.Fprintf(out, "  isolation claims:  %d (draft from summaries; do not paste bodies)\n", len(d.Isolation.Claims))
		for _, c := range d.Isolation.Claims {
			fmt.Fprintf(out, "    %s — %s\n", c.ID, c.Summary)
		}
		if len(d.Isolation.DraftHints.SuggestedProvides) > 0 {
			fmt.Fprintf(out, "  suggested provides: %s\n", strings.Join(d.Isolation.DraftHints.SuggestedProvides, ", "))
		}
	}
	if isolation && d.IsolationBudget != nil {
		b := d.IsolationBudget
		fmt.Fprintf(out, "  isolation bytes:   shared %d/%d, module %d/%d\n", b.SharedBytes, b.SharedBudget, b.ModuleBytes, b.ModuleBudget)
	}
	if integration && d.Integration != nil {
		fmt.Fprintf(out, "  neighbors:         %d\n", len(d.Integration.Neighbors))
		for _, n := range d.Integration.Neighbors {
			fmt.Fprintf(out, "    %s via %s — %s\n", n.Module, n.Via, n.Summary)
			for _, p := range n.Provides {
				fmt.Fprintf(out, "      %s — %s\n", p.ID, p.Summary)
			}
		}
		if !isolation {
			fmt.Fprintf(out, "  project claims:    %d\n", len(d.Integration.ProjectClaims))
			for _, e := range d.Integration.ProjectClaims {
				fmt.Fprintf(out, "    %s — %s\n", e.ID, e.Summary)
			}
		}
	}
}

// constitutionLine is the text report's one-word-plus-size roof line.
func constitutionLine(d manifest.ConstitutionDigest) string {
	if !d.Present {
		return "missing (" + d.Path + ")"
	}
	state := d.State
	if state == "" {
		state = d.Status
	}
	return fmt.Sprintf("%s, %d of %d words", state, d.Words, d.WordCap)
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
