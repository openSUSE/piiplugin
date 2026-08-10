package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	filterusername "github.com/openSUSE/piirplug/filter/username"
	"github.com/toon-format/toon-go"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/full"
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

	"github.com/openSUSE/piirplug/utils"
)

type ProcessInfo struct {
	Pid        int     `json:"pid" toon:"pid" jsonschema:"The unique process ID."`
	Name       string  `json:"name" toon:"name" jsonschema:"The executable name of the process."`
	Uid        uint32  `json:"uid" toon:"uid" jsonschema:"The owner's numeric user ID."`
	Username   string  `json:"username" toon:"username" jsonschema:"The owner's username."`
	MemoryB    uint64  `json:"memory_bytes" toon:"memory_bytes" jsonschema:"Resident set size memory in bytes."`
	VSizeB     uint64  `json:"vsize_bytes" toon:"vsize_bytes" jsonschema:"Virtual memory size in bytes."`
	CpuTimeSec float64 `json:"cpu_time_seconds" toon:"cpu_time_seconds" jsonschema:"Total CPU time (user + system) consumed in seconds."`
}

type GetProcessesResult struct {
	Processes string `json:"processes" jsonschema:"List of running processes in TOON format."`
}

const psFormat = "pid=,uid=,user=,rss=,vsz=,times=,comm="

const psColumns = 7

func getProcesses(ctx agent.Context, args struct{}) (GetProcessesResult, error) {
	out, err := exec.CommandContext(ctx, "ps", "-o", psFormat, "--ppid", "2", "-p", "2", "--deselect").Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return GetProcessesResult{}, fmt.Errorf("ps failed: %w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return GetProcessesResult{}, fmt.Errorf("failed to run ps: %w", err)
	}

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
		processes = append(processes, proc)
	}

	toonBytes, err := toon.Marshal(processes)
	if err != nil {
		return GetProcessesResult{}, fmt.Errorf("failed to format processes in TOON: %w", err)
	}

	return GetProcessesResult{Processes: string(toonBytes)}, nil
}

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
		Pid:        pid,
		Name:       strings.Join(fields[psColumns-1:], " "),
		Uid:        uint32(uid),
		Username:   fields[2],
		MemoryB:    rssKiB * 1024,
		VSizeB:     vszKiB * 1024,
		CpuTimeSec: cpuTimeSec,
	}, nil
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

	// Consume our own flags, the rest of the arguments belong to the launcher.
	disableUsernamePlugin, prompt, launcherArgs, err := utils.SplitOwnFlags(os.Args[1:])
	if err != nil {
		log.Fatalf("Failed to parse arguments: %v", err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "system_agent",
		Model:       model,
		Description: "Answers questions using an Ollama model.",
		Instruction: "You are a helpful assistant giving information about system you are running on.",
		Tools: []tool.Tool{
			processesTool,
		},
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Build plugin list
	var plugins []*plugin.Plugin
	if !disableUsernamePlugin {
		usernamePlugin, err := filterusername.NewUsernamePlugin()
		if err != nil {
			log.Fatalf("Failed to create username plugin: %v", err)
		}
		plugins = append(plugins, usernamePlugin)
	}

	config := &launcher.Config{
		AgentLoader:    agent.NewSingleLoader(a),
		SessionService: sessionService,
		PluginConfig: runner.PluginConfig{
			Plugins: plugins,
		},
	}

	l := full.NewLauncher()

	// A single prompt is answered directly, without starting a launcher.
	if prompt != "" {
		if err := runPrompt(ctx, config, prompt); err != nil {
			log.Fatalf("Run failed: %v\n", err)
		}
		return
	}

	// Without any argument there is nothing to run, so show how to run it.
	if len(launcherArgs) == 0 {
		fmt.Printf("Usage: %s [-p|--prompt PROMPT] [--disable-username-plugin] [LAUNCHER ARGUMENTS]\n\n", filepath.Base(os.Args[0]))
		fmt.Printf("  -p, --prompt PROMPT\n        answer a single prompt and exit\n")
		fmt.Printf("  --disable-username-plugin\n        run without the username PII filter\n\n")
		fmt.Println(l.CommandLineSyntax())
		return
	}

	if err := l.Execute(ctx, config, launcherArgs); err != nil {
		log.Fatalf("Run failed: %v\n", err)
	}
}

// runPrompt answers a single prompt with the configured agent and prints the
// response to stdout. It uses the same session service and plugins as the
// launcher, so the PII filters apply here as well.
func runPrompt(ctx context.Context, config *launcher.Config, prompt string) error {
	const appName, userID = "prompt_app", "prompt_user"

	resp, err := config.SessionService.Create(ctx, &session.CreateRequest{
		AppName: appName,
		UserID:  userID,
	})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	r, err := runner.New(runner.Config{
		AppName:        appName,
		Agent:          config.AgentLoader.RootAgent(),
		SessionService: config.SessionService,
		PluginConfig:   config.PluginConfig,
	})
	if err != nil {
		return fmt.Errorf("failed to create runner: %w", err)
	}

	msg := genai.NewContentFromText(prompt, genai.RoleUser)
	for event, err := range r.Run(ctx, userID, resp.Session.ID(), msg, agent.RunConfig{
		StreamingMode: agent.StreamingModeNone,
	}) {
		if err != nil {
			return err
		}
		if event.LLMResponse.Content == nil {
			continue
		}
		for _, part := range event.LLMResponse.Content.Parts {
			fmt.Print(part.Text)
		}
	}
	fmt.Println()

	return nil
}
