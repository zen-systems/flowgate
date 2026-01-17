package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zen-systems/flowgate/pkg/adapter"
	"github.com/zen-systems/flowgate/pkg/artifact"
	"github.com/zen-systems/flowgate/pkg/config"
	"github.com/zen-systems/flowgate/pkg/curator"
	"github.com/zen-systems/flowgate/pkg/curator/sources"
	"github.com/zen-systems/flowgate/pkg/gate"
	"github.com/zen-systems/flowgate/pkg/router"
)

var (
	configFile  string
	adapterFlag string
	modelFlag   string
	aliases     *config.ModelAliases
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "flowgate",
		Short: "AI orchestration system with intelligent routing and quality gates",
		Long: `Flowgate is an AI orchestration system that intelligently routes prompts
to the most appropriate LLM provider based on task type, and enforces
quality gates on outputs.`,
	}

	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "path to routing config file")

	rootCmd.AddCommand(askCmd())
	rootCmd.AddCommand(routesCmd())
	rootCmd.AddCommand(modelsCmd())
	rootCmd.AddCommand(validateCmd())
	rootCmd.AddCommand(runCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func askCmd() *cobra.Command {
	var deepFlag bool
	var gateFlag string
	var maxRetries int

	cmd := &cobra.Command{
		Use:   "ask [prompt]",
		Short: "Send a prompt to the appropriate LLM",
		Long: `Routes the prompt to the best LLM based on task type detection,
or use --adapter and --model to override.

Use --deep for complex queries that benefit from context curation.
The curator will analyze your query, gather relevant information from
multiple sources, and synthesize an optimal context before responding.

Use --gate to enable quality gates with automatic repair loops.
If the output fails quality checks, the model will be prompted to fix
the issues and regenerate (up to --retries attempts).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := args[0]

			cfg, err := loadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adapters, err := createAdapters(cfg)
			if err != nil {
				return fmt.Errorf("failed to create adapters: %w", err)
			}

			if len(adapters) == 0 {
				return fmt.Errorf("no adapters available - please set API keys")
			}

			r := router.NewRouter(adapters, cfg.RoutingConfig, router.WithAliases(aliases))

			var targetAdapter adapter.Adapter
			var model string

			if adapterFlag != "" {
				a, ok := r.GetAdapter(adapterFlag)
				if !ok {
					return fmt.Errorf("adapter %q not available", adapterFlag)
				}
				targetAdapter = a
				if modelFlag != "" {
					// Resolve alias if provided
					model = modelFlag
					if aliases != nil {
						model = aliases.Resolve(model)
					}
				} else {
					models := a.Models()
					if len(models) > 0 {
						model = models[0]
					}
				}
			} else {
				targetAdapter, model = r.Route(prompt)
			}

			if targetAdapter == nil {
				return fmt.Errorf("no adapter available")
			}

			// Wrap with Curator if --deep flag is set
			var finalAdapter adapter.Adapter = targetAdapter
			if deepFlag {
				cur, err := createCurator(targetAdapter, cfg)
				if err != nil {
					return fmt.Errorf("failed to create curator: %w", err)
				}
				finalAdapter = cur
				fmt.Fprintf(os.Stderr, "Using curator with %s/%s\n", targetAdapter.Name(), model)
			} else {
				fmt.Fprintf(os.Stderr, "Routing to %s/%s\n", targetAdapter.Name(), model)
			}

			// Initialize quality gate if requested
			var qualityGate gate.Gate
			if gateFlag != "" {
				qualityGate = gate.NewHollowCheckGate("", gateFlag)
				fmt.Fprintf(os.Stderr, "Quality gate enabled: %s\n", qualityGate.Name())
			}

			// Generate with optional repair loop
			var finalArtifact *artifact.Artifact
			currentPrompt := prompt

			for attempt := 1; attempt <= maxRetries; attempt++ {
				if attempt > 1 {
					fmt.Fprintf(os.Stderr, "Attempt %d/%d: Repairing output...\n", attempt, maxRetries)
				}

				// Generate
				art, err := finalAdapter.Generate(context.Background(), model, currentPrompt)
				if err != nil {
					return fmt.Errorf("generation failed: %w", err)
				}
				finalArtifact = art

				// Skip gate check if no gate configured
				if qualityGate == nil {
					break
				}

				// Check quality
				result, err := qualityGate.Evaluate(art)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Gate check failed: %v\n", err)
					break // Fail open if gate breaks
				}

				if result.Passed {
					fmt.Fprintf(os.Stderr, "Quality gate passed (score: %d)\n", result.Score)
					break
				}

				// Gate failed
				fmt.Fprintf(os.Stderr, "Quality gate failed (score: %d, %d violations)\n",
					result.Score, len(result.Violations))

				if attempt == maxRetries {
					fmt.Fprintf(os.Stderr, "Max retries reached. Final output did not pass gate.\n")
					break
				}

				// Construct repair prompt
				currentPrompt = buildRepairPrompt(prompt, art.Content, result)
			}

			fmt.Println(finalArtifact.Content)
			return nil
		},
	}

	cmd.Flags().StringVar(&adapterFlag, "adapter", "", "override adapter (anthropic, openai, google)")
	cmd.Flags().StringVar(&modelFlag, "model", "", "override model")
	cmd.Flags().BoolVar(&deepFlag, "deep", false, "use context curator for complex queries")
	cmd.Flags().StringVar(&gateFlag, "gate", "", "enable quality gate with contract file (e.g., hollowcheck.yaml)")
	cmd.Flags().IntVar(&maxRetries, "retries", 3, "max repair attempts when gate fails")

	return cmd
}

// buildRepairPrompt constructs a prompt that includes feedback for the model to fix issues.
func buildRepairPrompt(originalPrompt, previousOutput string, result *gate.GateResult) string {
	var feedback strings.Builder

	feedback.WriteString("The previous implementation failed quality checks.\n\n")
	feedback.WriteString("## Violations to Fix:\n")

	for _, v := range result.Violations {
		feedback.WriteString(fmt.Sprintf("- [%s] %s at %s: %s\n",
			v.Severity, v.Rule, v.Location, v.Message))
	}

	if len(result.RepairHints) > 0 {
		feedback.WriteString("\n## Repair Instructions:\n")
		for _, hint := range result.RepairHints {
			feedback.WriteString(fmt.Sprintf("- %s\n", hint))
		}
	}

	feedback.WriteString("\n## CRITICAL REQUIREMENTS:\n")
	feedback.WriteString("- Do NOT use TODO, FIXME, or placeholder comments\n")
	feedback.WriteString("- Do NOT use panic(\"not implemented\") or stub implementations\n")
	feedback.WriteString("- Do NOT use mock data like example.com, test@test.com, or sequential IDs\n")
	feedback.WriteString("- Write the COMPLETE, WORKING implementation\n")

	return fmt.Sprintf(`%s

## Previous Output (Failed Quality Gate):
%s

Please rewrite the code to fix ALL issues listed above.`, feedback.String(), previousOutput)
}

// createCurator initializes the Curator with available sources.
func createCurator(targetAdapter adapter.Adapter, cfg *config.Config) (*curator.Curator, error) {
	// Build curator options based on available resources
	opts := []curator.CuratorOption{
		curator.WithConfig(curator.CuratorConfig{
			TargetAdapter:       targetAdapter.Name(),
			TargetModel:         "", // Will use model from Generate call
			AnalysisModel:       "", // Will use same as target
			ContextBudget:       100000,
			ConfidenceThreshold: 0.7,
			EnabledSources:      []curator.SourceType{curator.SourceFilesystem, curator.SourceMemory, curator.SourceArtifacts},
			MaxGatherParallel:   10,
			MaxGatherPerSource:  20,
			Debug:               true,
		}),
		curator.WithLogger(func(format string, args ...any) {
			log.Printf(format, args...)
		}),
	}

	// Add filesystem source (current directory)
	cwd, err := os.Getwd()
	if err == nil {
		opts = append(opts, curator.WithFilesystemSource(cwd))
	}

	// Add memory source for conversation tracking
	opts = append(opts, curator.WithMemorySource(sources.NewMemorySource()))

	// Add artifact source for tracking outputs
	opts = append(opts, curator.WithArtifactSource(sources.NewArtifactSource()))

	// Add web source if Tavily API key is available
	if os.Getenv("TAVILY_API_KEY") != "" {
		opts = append(opts, curator.WithWebSource())
		log.Println("[curator] Web search enabled (Tavily)")
	}

	// Use the target adapter for both generation and analysis
	// This keeps it simple - use the same model for everything
	return curator.NewCurator(targetAdapter, targetAdapter, opts...)
}

func routesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "routes",
		Short: "Show current routing rules",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "TASK TYPE\tADAPTER\tMODEL\tTRIGGERS")

			// Sort task types for consistent output
			var taskTypes []string
			for name := range cfg.RoutingConfig.TaskTypes {
				taskTypes = append(taskTypes, name)
			}
			sort.Strings(taskTypes)

			for _, name := range taskTypes {
				tt := cfg.RoutingConfig.TaskTypes[name]
				triggers := ""
				for i, t := range tt.Triggers {
					if i > 0 {
						triggers += ", "
					}
					triggers += t
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, tt.Adapter, tt.Model, triggers)
			}

			fmt.Fprintln(w)
			fmt.Fprintf(w, "DEFAULT\t%s\t%s\t-\n",
				cfg.RoutingConfig.Default.Adapter,
				cfg.RoutingConfig.Default.Model)

			return w.Flush()
		},
	}
}

func modelsCmd() *cobra.Command {
	var resolveFlag bool
	var validateFlag bool

	cmd := &cobra.Command{
		Use:   "models",
		Short: "List available adapters, models, and aliases",
		Long: `Lists adapters and their available models.

Use --resolve to show aliases and what they resolve to.
Use --validate to check all aliases in routing.yaml resolve to valid models.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Handle --resolve flag
			if resolveFlag {
				return showAliases()
			}

			// Handle --validate flag
			if validateFlag {
				return validateAliases(cfg)
			}

			// Default: show providers and models from config
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "PROVIDER\tMODELS\tSTATUS")

			// Get provider list from aliases config
			providers := aliases.ListProviders()
			if len(providers) == 0 {
				// Fall back to known providers
				providers = []string{"anthropic", "deepseek", "google", "openai"}
			}

			for _, provider := range providers {
				models := formatList(aliases.GetProviderModels(provider))
				status := "no key"
				if cfg.HasAdapter(provider) {
					status = "ready"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", provider, models, status)
			}

			return w.Flush()
		},
	}

	cmd.Flags().BoolVar(&resolveFlag, "resolve", false, "show aliases and what they resolve to")
	cmd.Flags().BoolVar(&validateFlag, "validate", false, "check all aliases in routing.yaml resolve to valid models")

	return cmd
}

func showAliases() error {
	if aliases == nil {
		fmt.Println("No model aliases configured.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ALIAS\tMODEL\tPROVIDER")

	// Sort aliases for consistent output
	aliasMap := aliases.ListAliases()
	var aliasNames []string
	for name := range aliasMap {
		aliasNames = append(aliasNames, name)
	}
	sort.Strings(aliasNames)

	for _, alias := range aliasNames {
		model := aliasMap[alias]
		provider := aliases.GetProviderForModel(model)
		fmt.Fprintf(w, "%s\t%s\t%s\n", alias, model, provider)
	}

	return w.Flush()
}

func validateAliases(cfg *config.Config) error {
	if aliases == nil {
		fmt.Println("No model aliases configured - nothing to validate.")
		return nil
	}

	errors := aliases.ValidateRoutingConfig(cfg.RoutingConfig)
	if len(errors) == 0 {
		fmt.Println("All models in routing.yaml are valid.")
		return nil
	}

	fmt.Fprintf(os.Stderr, "Found %d validation errors:\n", len(errors))
	for _, err := range errors {
		fmt.Fprintf(os.Stderr, "  - %s\n", err)
	}
	return fmt.Errorf("validation failed")
}

func formatList(items []string) string {
	if len(items) == 0 {
		return ""
	}
	result := items[0]
	for i := 1; i < len(items); i++ {
		result += ", " + items[i]
	}
	return result
}

func validateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [pipeline.yaml]",
		Short: "Validate a pipeline manifest",
		Long:  "Phase 2: Validates pipeline YAML without executing.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Pipeline validation is not yet implemented (phase 2)")
			return nil
		},
	}
}

func runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [pipeline.yaml]",
		Short: "Execute a pipeline",
		Long:  "Phase 2: Runs a pipeline with the specified input.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Pipeline execution is not yet implemented (phase 2)")
			return nil
		},
	}

	cmd.Flags().String("input", "", "input file for the pipeline")

	return cmd
}

func loadConfig() (*config.Config, error) {
	var cfg *config.Config
	var err error

	if configFile != "" {
		cfg, err = config.LoadWithRoutingFile(configFile)
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		return nil, err
	}

	// Load model aliases
	aliases, _ = config.LoadAliasesWithFallback("configs/models.yaml")

	return cfg, nil
}

func createAdapters(cfg *config.Config) (map[string]adapter.Adapter, error) {
	adapters := make(map[string]adapter.Adapter)

	if cfg.AnthropicAPIKey != "" {
		a, err := adapter.NewAnthropicAdapter(cfg.AnthropicAPIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create anthropic adapter: %w", err)
		}
		adapters["anthropic"] = a
	}

	if cfg.OpenAIAPIKey != "" {
		a, err := adapter.NewOpenAIAdapter(cfg.OpenAIAPIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create openai adapter: %w", err)
		}
		adapters["openai"] = a
	}

	if cfg.GoogleAPIKey != "" {
		a, err := adapter.NewGoogleAdapter(cfg.GoogleAPIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create google adapter: %w", err)
		}
		adapters["google"] = a
	}

	if cfg.DeepSeekAPIKey != "" {
		a, err := adapter.NewDeepSeekAdapter(cfg.DeepSeekAPIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create deepseek adapter: %w", err)
		}
		adapters["deepseek"] = a
	}

	return adapters, nil
}
