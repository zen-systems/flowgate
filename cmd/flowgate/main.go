package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zen-systems/flowgate/pkg/adapter"
	"github.com/zen-systems/flowgate/pkg/archive"
	"github.com/zen-systems/flowgate/pkg/attest"
	"github.com/zen-systems/flowgate/pkg/bridge"
	"github.com/zen-systems/flowgate/pkg/config"
	"github.com/zen-systems/flowgate/pkg/crypto"
	"github.com/zen-systems/flowgate/pkg/curator"
	"github.com/zen-systems/flowgate/pkg/curator/sources"
	"github.com/zen-systems/flowgate/pkg/pipeline"
	"github.com/zen-systems/flowgate/pkg/policy"
	"github.com/zen-systems/flowgate/pkg/router"
	"github.com/zen-systems/provenance-gate"
	"github.com/zen-systems/vtp-runtime/orchestrator"
	"github.com/zen-systems/vtp-runtime/registry"
)

var (
	configFile  string
	adapterFlag string
	modelFlag   string
	aliases     *config.ModelAliases

	// Global VTP components
	vtpOrchestrator *orchestrator.Orchestrator
	vtpListener     *bridge.Listener
)

func main() {
	// Initialize VTP Layer
	initVTP()
	defer func() {
		if vtpListener != nil {
			vtpListener.Close()
		}
	}()

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
	rootCmd.AddCommand(attestCmd())
	rootCmd.AddCommand(verifyCmd())

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

			p := &pipeline.Pipeline{
				Name: "ask",
				Stages: []*pipeline.Stage{
					{
						Name:       "ask",
						Adapter:    finalAdapter.Name(),
						Model:      model,
						Prompt:     "{{ .Input }}",
						MaxRetries: maxRetries,
					},
				},
				Adapters: map[string]adapter.Adapter{finalAdapter.Name(): finalAdapter},
			}

			if gateFlag != "" {
				p.Gates = map[string]pipeline.GateDefinition{
					"hollowcheck": {
						Type:         "hollowcheck",
						ContractPath: gateFlag,
					},
				}
				p.Stages[0].Gates = []string{"hollowcheck"}
			}

			result, err := pipeline.Run(context.Background(), p, pipeline.RunOptions{Input: prompt, RoutingConfig: cfg.RoutingConfig})
			if err != nil {
				return err
			}

			fmt.Println(result.Stages["ask"].Artifact.Content)
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

// createCurator initializes the Curator with available sources.
func createCurator(targetAdapter adapter.Adapter, cfg *config.Config) (*curator.Curator, error) {
	opts := []curator.CuratorOption{
		curator.WithConfig(curator.CuratorConfig{
			TargetAdapter:       targetAdapter.Name(),
			TargetModel:         "",
			AnalysisModel:       "",
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

	cwd, err := os.Getwd()
	if err == nil {
		opts = append(opts, curator.WithFilesystemSource(cwd))
	}

	opts = append(opts, curator.WithMemorySource(sources.NewMemorySource()))
	opts = append(opts, curator.WithArtifactSource(sources.NewArtifactSource()))

	if os.Getenv("TAVILY_API_KEY") != "" {
		opts = append(opts, curator.WithWebSource())
		log.Println("[curator] Web search enabled (Tavily)")
	}

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

			if resolveFlag {
				return showAliases()
			}

			if validateFlag {
				return validateAliases(cfg)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "PROVIDER\tMODELS\tSTATUS")

			providers := aliases.ListProviders()
			if len(providers) == 0 {
				providers = []string{"anthropic", "deepseek", "google", "openai", "mock"}
			}

			for _, provider := range providers {
				models := formatList(aliases.GetProviderModels(provider))
				status := "no key"
				if cfg.HasAdapter(provider) || provider == "mock" {
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
		Long:  "Validates pipeline YAML without executing.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := pipeline.LoadManifest(args[0])
			if err != nil {
				return err
			}
			if err := p.Validate(); err != nil {
				return err
			}
			fmt.Println("Pipeline manifest is valid.")
			return nil
		},
	}
}

func runCmd() *cobra.Command {
	var pipelineFile string
	var inputFlag string
	var workspaceFlag string
	var outFlag string
	var applyFlag bool
	var approveFlag bool
	var maxBudgetUSD float64

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Execute a pipeline",
		Long:  "Runs a pipeline with the specified input.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if pipelineFile == "" {
				return fmt.Errorf("pipeline file is required")
			}

			input := inputFlag
			if input == "" {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read stdin: %w", err)
				}
				input = strings.TrimSpace(string(data))
			}

			p, err := pipeline.LoadManifest(pipelineFile)
			if err != nil {
				return err
			}

			cfg, err := loadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adapters, err := createAdapters(cfg)
			if err != nil {
				return fmt.Errorf("failed to create adapters: %w", err)
			}
			p.Adapters = adapters

			result, err := pipeline.Run(context.Background(), p, pipeline.RunOptions{
				Input:           input,
				WorkspacePath:   workspaceFlag,
				EvidenceDir:     outFlag,
				PipelinePath:    pipelineFile,
				RoutingConfig:   cfg.RoutingConfig,
				MaxBudgetUSD:    maxBudgetUSD,
				ApplyForReal:    applyFlag,
				ApplyApproved:   approveFlag,
				VTPOrchestrator: vtpOrchestrator, // Pass the global orchestrator
			})
			if err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Run complete. Evidence: %s\n", result.EvidenceDir)
			return nil
		},
	}

	cmd.Flags().StringVarP(&pipelineFile, "file", "f", "", "pipeline manifest path (required)")
	cmd.Flags().StringVarP(&inputFlag, "input", "i", "", "input string for the pipeline (defaults to stdin)")
	cmd.Flags().StringVar(&workspaceFlag, "workspace", "", "workspace path for apply/gates")
	cmd.Flags().StringVar(&outFlag, "out", "", "evidence output base directory")
	cmd.Flags().BoolVar(&applyFlag, "apply", false, "apply changes to the real workspace")
	cmd.Flags().BoolVar(&approveFlag, "yes", false, "approve applying changes to the real workspace")
	cmd.Flags().Float64Var(&maxBudgetUSD, "max-budget-usd", 0, "maximum USD budget for adapter calls (0 disables)")

	return cmd
}

func attestCmd() *cobra.Command {
	var runDir string
	var stageName string
	var outFile string

	cmd := &cobra.Command{
		Use:   "attest",
		Short: "Export a v0 attestation for a stage",
		RunE: func(cmd *cobra.Command, args []string) error {
			if runDir == "" || stageName == "" || outFile == "" {
				return fmt.Errorf("--run, --stage, and --out are required")
			}

			attestation, err := attest.BuildAttestation(runDir, stageName)
			if err != nil {
				return err
			}

			data, err := json.MarshalIndent(attestation, "", "  ")
			if err != nil {
				return err
			}
			return os.WriteFile(outFile, data, 0644)
		},
	}

	cmd.Flags().StringVar(&runDir, "run", "", "run directory containing evidence")
	cmd.Flags().StringVar(&stageName, "stage", "", "stage name to attest")
	cmd.Flags().StringVar(&outFile, "out", "", "output file path")

	return cmd
}

func verifyCmd() *cobra.Command {
	var attestationPath string
	var runDir string

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify an attestation against a run directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			if attestationPath == "" || runDir == "" {
				return fmt.Errorf("--attestation and --run are required")
			}

			if err := attest.VerifyAttestationFile(attestationPath, runDir); err != nil {
				return err
			}

			fmt.Fprintln(os.Stdout, "Attestation verified.")
			return nil
		},
	}

	cmd.Flags().StringVar(&attestationPath, "attestation", "", "attestation file path")
	cmd.Flags().StringVar(&runDir, "run", "", "run directory containing evidence")

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

	adapters["mock"] = adapter.NewMockAdapter()

	return adapters, nil
}

func initVTP() {
	// 1. Initialize Registry
	reg := registry.NewRegistry()

	// Register Provenance Gate
	provGate := provenance.New(provenance.Config{
		TavilyAPIKey: os.Getenv("TAVILY_API_KEY"),
	})
	if err := reg.Register(provGate); err != nil {
		log.Printf("Failed to register provenance gate: %v", err)
	}

	// 2.5 Initialize Archive
	store, err := archive.NewStore("") // Use default ~/.flowgate/archive
	if err != nil {
		log.Printf("Failed to initialize archive: %v", err)
	}

	// 2.6 Initialize Policy & Signer
	policyReg := policy.NewRegistry()
	signer, err := crypto.NewSigner("flowgate-cli")
	if err != nil {
		log.Printf("Failed to initialize signer: %v", err)
	}

	// 3. Initialize Orchestrator
	orchCfg := orchestrator.OrchestratorConfig{
		Registry:       reg,
		SignerID:       "flowgate-cli",
		Archive:        store,
		PolicyRegistry: policyReg,
		Signer:         signer,
	}
	vtpOrchestrator = orchestrator.NewOrchestrator(orchCfg)

	// 4. Connect to Zenedge Kernel via Shared Memory
	// Path defined in zenedge_bridge.py is /dev/shm/zenedge.shm
	shmPath := "/dev/shm/zenedge.shm"
	// Fallback for local testing if not exists
	if _, err := os.Stat(shmPath); os.IsNotExist(err) {
		// Log warning but proceed
	}

	l, err := bridge.NewListener(shmPath, vtpOrchestrator)
	if err != nil {
		log.Printf("[VTP] Warning: Failed to connect to Zenedge Bridge: %v", err)
	} else {
		vtpListener = l
		log.Printf("[VTP] Connected to Zenedge Kernel at %s", shmPath)
		// 4. Start Listener
		go vtpListener.ListenForSignals()
	}
}
