package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	filterusername "github.com/openSUSE/piiplug/filter/username"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/console"
	"google.golang.org/adk/v2/cmd/launcher/universal"
	"google.golang.org/adk/v2/cmd/launcher/web"
	"google.golang.org/adk/v2/cmd/launcher/web/a2a"
	"google.golang.org/adk/v2/cmd/launcher/web/api"
	"google.golang.org/adk/v2/cmd/launcher/web/triggers/eventarc"
	"google.golang.org/adk/v2/cmd/launcher/web/triggers/pubsub"
	"google.golang.org/adk/v2/cmd/launcher/web/webui"
	"google.golang.org/adk/v2/model/openaimodel"
	"google.golang.org/adk/v2/plugin"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/session/database"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
	"google.golang.org/genai"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ProcessInfo represents details of a running process.
type ProcessInfo struct {
	Pid        int     `json:"pid" jsonschema:"The unique process ID."`
	Name       string  `json:"name" jsonschema:"The executable name of the process."`
	Uid        uint32  `json:"uid" jsonschema:"The owner's numeric user ID."`
	Username   string  `json:"username" jsonschema:"The owner's username."`
	MemoryB    uint64  `json:"memory_bytes" jsonschema:"Resident set size memory in bytes."`
	VSizeB     uint64  `json:"vsize_bytes" jsonschema:"Virtual memory size in bytes."`
	CpuTimeSec float64 `json:"cpu_time_seconds" jsonschema:"Total CPU time (user + system) consumed in seconds."`
}

// GetProcessesArgs is the arguments for the get_processes tool.
type GetProcessesArgs struct {
	FilterName string `json:"filter_name,omitempty" jsonschema:"An optional case-insensitive substring filter for process names."`
}

// GetProcessesResult is the response of the get_processes tool.
type GetProcessesResult struct {
	Processes []ProcessInfo `json:"processes" jsonschema:"List of running processes."`
}

// psFormat are the columns requested from ps, in order and without headers.
// comm comes last because a command name may contain spaces.
const psFormat = "pid=,uid=,user=,rss=,vsz=,times=,comm="

// psColumns is the number of columns psFormat asks for.
const psColumns = 7

func getProcesses(ctx agent.Context, args GetProcessesArgs) (GetProcessesResult, error) {
	out, err := exec.CommandContext(ctx, "ps", "-o", psFormat, "--ppid", "2", "-p", "2", "--deselect").Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return GetProcessesResult{}, fmt.Errorf("ps failed: %w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return GetProcessesResult{}, fmt.Errorf("failed to run ps: %w", err)
	}

	filter := strings.ToLower(args.FilterName)
	var processes []ProcessInfo

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		proc, err := parsePsLine(line)
		if err != nil {
			// A single unparsable line should not fail the whole listing.
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(proc.Name), filter) {
			continue
		}
		processes = append(processes, proc)
	}

	return GetProcessesResult{Processes: processes}, nil
}

// parsePsLine turns one line of ps output, as described by psFormat, into a
// ProcessInfo. ps reports rss and vsz in kibibytes and times in whole seconds.
func parsePsLine(line string) (ProcessInfo, error) {
	fields := strings.Fields(line)
	if len(fields) < psColumns {
		return ProcessInfo{}, fmt.Errorf("expected %d columns, got %d in %q", psColumns, len(fields), line)
	}

	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return ProcessInfo{}, fmt.Errorf("invalid pid %q: %w", fields[0], err)
	}
	uid, err := strconv.ParseUint(fields[1], 10, 32)
	if err != nil {
		return ProcessInfo{}, fmt.Errorf("invalid uid %q: %w", fields[1], err)
	}
	rssKiB, err := strconv.ParseUint(fields[3], 10, 64)
	if err != nil {
		return ProcessInfo{}, fmt.Errorf("invalid rss %q: %w", fields[3], err)
	}
	vszKiB, err := strconv.ParseUint(fields[4], 10, 64)
	if err != nil {
		return ProcessInfo{}, fmt.Errorf("invalid vsz %q: %w", fields[4], err)
	}
	cpuTimeSec, err := strconv.ParseFloat(fields[5], 64)
	if err != nil {
		return ProcessInfo{}, fmt.Errorf("invalid cpu time %q: %w", fields[5], err)
	}

	return ProcessInfo{
		Pid: pid,
		// Everything past the fixed columns belongs to the command name.
		Name:       strings.Join(fields[psColumns-1:], " "),
		Uid:        uint32(uid),
		Username:   fields[2],
		MemoryB:    rssKiB * 1024,
		VSizeB:     vszKiB * 1024,
		CpuTimeSec: cpuTimeSec,
	}, nil
}

type promptLauncher struct {
	prompt string
	flags  *flag.FlagSet
}

func newPromptLauncher() launcher.SubLauncher {
	fs := flag.NewFlagSet("prompt", flag.ContinueOnError)
	pl := &promptLauncher{flags: fs}
	fs.StringVar(&pl.prompt, "p", "", "The prompt to run the agent with")
	return pl
}

func (l *promptLauncher) Keyword() string {
	return "prompt"
}

func (l *promptLauncher) Parse(args []string) ([]string, error) {
	err := l.flags.Parse(args)
	return l.flags.Args(), err
}

func (l *promptLauncher) CommandLineSyntax() string {
	return "  -p string\n        The prompt to run the agent with"
}

func (l *promptLauncher) SimpleDescription() string {
	return "runs the agent with a single prompt and exits"
}

func (l *promptLauncher) Run(ctx context.Context, config *launcher.Config) error {
	if l.prompt == "" {
		// If no prompt is specified, default to the interactive console launcher
		return console.NewLauncher().Run(ctx, config)
	}

	userID, appName := "console_user", "console_app"
	resp, err := config.SessionService.Create(ctx, &session.CreateRequest{
		AppName: appName,
		UserID:  userID,
	})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	sess := resp.Session

	r, err := runner.New(runner.Config{
		AppName:        appName,
		Agent:          config.AgentLoader.RootAgent(),
		SessionService: config.SessionService,
		PluginConfig:   config.PluginConfig,
	})
	if err != nil {
		return fmt.Errorf("failed to create runner: %w", err)
	}

	userMsg := genai.NewContentFromText(l.prompt, genai.RoleUser)

	for event, err := range r.Run(ctx, userID, sess.ID(), userMsg, agent.RunConfig{
		StreamingMode: agent.StreamingModeSSE,
	}) {
		if err != nil {
			fmt.Printf("\nAGENT_ERROR: %v\n", err)
		} else {
			if event.LLMResponse.Content != nil {
				for _, p := range event.LLMResponse.Content.Parts {
					fmt.Print(p.Text)
				}
			}
		}
	}
	fmt.Println()
	return nil
}

func main() {
	ctx := context.Background()
	baseURL := os.Getenv("OLLAMA_URL")
	if baseURL == "" {
		baseURL = "http://localhost:3741/v1"
	}
	modelName := os.Getenv("OLLAMA_MODEL")
	if modelName == "" {
		modelName = "default:latest"
	}

	model, err := openaimodel.NewModel(ctx, modelName, &openaimodel.ClientConfig{
		BaseURL: baseURL,
	})
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// Define the get_processes tool
	processesTool, err := functiontool.New(
		functiontool.Config{
			Name:        "get_processes",
			Description: "Retrieves a list of all running processes on the system, including PID, process name, owner's UID and username, resident/virtual memory used, and CPU time.",
		},
		getProcesses,
	)
	if err != nil {
		log.Fatalf("Failed to create get_processes tool: %v", err)
	}

	sessionService, err := database.NewSessionService(
		sqlite.Open("sessions.db"),
		&gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		},
	)
	if err != nil {
		log.Fatalf("Failed to create session service: %v", err)
	}
	if err := database.AutoMigrate(sessionService); err != nil {
		log.Fatalf("Failed to auto-migrate database: %v", err)
	}
	usernamePlugin, err := filterusername.NewUsernamePlugin()
	if err != nil {
		log.Fatalf("Failed to create username plugin: %v", err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "system_agent",
		Model:       model,
		Description: "Answers questions using an Ollama model.",
		Instruction: "You are a helpful assistant.",
		Tools: []tool.Tool{
			processesTool,
		},
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	config := &launcher.Config{
		AgentLoader:    agent.NewSingleLoader(a),
		SessionService: sessionService,
		PluginConfig: runner.PluginConfig{
			Plugins: []*plugin.Plugin{
				usernamePlugin,
			},
		},
	}

	l := universal.NewLauncher(
		newPromptLauncher(),
		console.NewLauncher(),
		web.NewLauncher(webui.NewLauncher(), a2a.NewLauncher(), pubsub.NewLauncher(), eventarc.NewLauncher(), api.NewLauncher()),
	)

	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"console"}
	}
	if err := l.Execute(ctx, config, args); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
