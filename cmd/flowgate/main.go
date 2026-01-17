package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zen-systems/flowgate/pkg/adapter"
	"github.com/zen-systems/flowgate/pkg/config"
	"github.com/zen-systems/flowgate/pkg/router"
)

var (
	configFile  string
	adapterFlag string
	modelFlag   string
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
	cmd := &cobra.Command{
		Use:   "ask [prompt]",
		Short: "Send a prompt to the appropriate LLM",
		Long: `Routes the prompt to the best LLM based on task type detection,
or use --adapter and --model to override.`,
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

			r := router.NewRouter(adapters, cfg.RoutingConfig)

			var result *adapter.Adapter
			var model string

			if adapterFlag != "" {
				a, ok := r.GetAdapter(adapterFlag)
				if !ok {
					return fmt.Errorf("adapter %q not available", adapterFlag)
				}
				result = &a
				if modelFlag != "" {
					model = modelFlag
				} else {
					models := a.Models()
					if len(models) > 0 {
						model = models[0]
					}
				}
			} else {
				a, m := r.Route(prompt)
				result = &a
				model = m
			}

			if result == nil || *result == nil {
				return fmt.Errorf("no adapter available")
			}

			fmt.Fprintf(os.Stderr, "Routing to %s/%s\n", (*result).Name(), model)

			artifact, err := (*result).Generate(context.Background(), model, prompt)
			if err != nil {
				return fmt.Errorf("generation failed: %w", err)
			}

			fmt.Println(artifact.Content)
			return nil
		},
	}

	cmd.Flags().StringVar(&adapterFlag, "adapter", "", "override adapter (anthropic, openai, google)")
	cmd.Flags().StringVar(&modelFlag, "model", "", "override model")

	return cmd
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
	return &cobra.Command{
		Use:   "models",
		Short: "List available adapters and models",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adapters, err := createAdapters(cfg)
			if err != nil {
				return fmt.Errorf("failed to create adapters: %w", err)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ADAPTER\tMODELS")

			// Sort adapter names for consistent output
			var names []string
			for name := range adapters {
				names = append(names, name)
			}
			sort.Strings(names)

			for _, name := range names {
				a := adapters[name]
				models := ""
				for i, m := range a.Models() {
					if i > 0 {
						models += ", "
					}
					models += m
				}
				fmt.Fprintf(w, "%s\t%s\n", name, models)
			}

			if len(adapters) == 0 {
				fmt.Fprintln(os.Stderr, "\nNo adapters available. Set API keys:")
				fmt.Fprintln(os.Stderr, "  ANTHROPIC_API_KEY - for Claude models")
				fmt.Fprintln(os.Stderr, "  OPENAI_API_KEY    - for OpenAI models")
				fmt.Fprintln(os.Stderr, "  GOOGLE_API_KEY    - for Gemini models")
				fmt.Fprintln(os.Stderr, "  DEEPSEEK_API_KEY  - for DeepSeek models")
			}

			return w.Flush()
		},
	}
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
	if configFile != "" {
		return config.LoadWithRoutingFile(configFile)
	}
	return config.Load()
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
