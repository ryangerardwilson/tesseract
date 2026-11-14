package tmux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManagedIndexKeyReadsFragmentConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "tmux.conf")
	fragmentDir := filepath.Join(dir, "tmux.conf.d")
	if err := os.MkdirAll(fragmentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("source-file ~/.config/tmux/tmux.conf.d/*.conf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fragmentDir, "20-managed-picker.conf"), []byte(`bind -n "F8" run-shell "TMUX_CLIENT_TTY='#{client_tty}' \"/tmp/tesseract\" open picker >/dev/null 2>&1"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := ManagedIndexKey(configPath); got != "F8" {
		t.Fatalf("ManagedIndexKey() = %q, want F8", got)
	}
}

func TestPaneTextShowsWorking(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{
			name: "current codex working status",
			text: "\n\u2022 Working (1m 39s \u2022 esc to interrupt)\n\n" +
				"\u203a Run /review on my current changes\n\n" +
				"gpt-5.5 xhigh fast \u00b7 ~\n",
			want: true,
		},
		{
			name: "working status without elapsed time",
			text: "\n\u2022 Working\n\n" +
				"\u203a Run tests\n\n" +
				"gpt-5.5 xhigh fast \u00b7 ~\n",
			want: true,
		},
		{
			name: "codex through GLM working status",
			text: "\n\u2022 Working (1h 35m 51s \u2022 esc to interrupt)\n\n" +
				"\u203a Ask Codex to do anything\n\n" +
				"  glm-5.3 high \u00b7 ~/Work/project\n",
			want: true,
		},
		{
			name: "codex planning status",
			text: "\n\u2022 Planning targeted fil…\n\n" +
				"\u203a Ask Codex to do anything\n\n" +
				"  gpt-5.6-sol high fast \u00b7 ~\n",
			want: true,
		},
		{
			name: "glm planning status",
			text: "\n\u2022 Planning the church listings\n" +
				"\u203a Ask Codex to do anything\n" +
				"  glm-5.3-flash high \u00b7 ~/Apps/boxes\n",
			want: true,
		},
		{
			name: "planning prose is not status",
			text: "\nI am planning the release notes.\n\n" +
				"\u203a next command\n",
			want: false,
		},
		{
			name: "codex reviewing status",
			text: "\n\u2022 Reviewing the listings…\n\n" +
				"\u203a Ask Codex to do anything\n\n" +
				"  gpt-5.6-sol high fast \u00b7 ~\n",
			want: true,
		},
		{
			name: "glm drafting status",
			text: "\n\u2022 Drafting the reply\n" +
				"\u203a Ask Codex to do anything\n" +
				"  glm-5.3-flash high \u00b7 ~/Apps/boxes\n",
			want: true,
		},
		{
			name: "codex activity header",
			text: "\n\u2022 Mapping the app structure\n\n" +
				"\u203a Ask Codex to do anything\n\n" +
				"  gpt-5.6-sol high \u00b7 ~\n",
			want: true,
		},
		{
			name: "finished bullet answer is not status",
			text: "\n\u2022 Yes. It is deployed to DEV and verified:\n" +
				"\u203a Ask Codex to do anything\n" +
				"  gpt-5.6-sol high fast \u00b7 ~\n",
			want: false,
		},
		{
			name: "completed tool bullet is not status",
			text: "\n\u2022 Ran git status\n" +
				"  \u2514 ## main\n" +
				"  Worked for 5m 32s \u00b7 done\n" +
				"\u203a Ask Codex to do anything\n" +
				"  glm-5.3-flash high \u00b7 ~\n",
			want: false,
		},
		{
			name: "working status with current queue footer",
			text: "\n\u2022 Working\n\n" +
				"\u203a fix the tmux activity animation\n\n" +
				"tab to queue message 81% context left\n",
			want: true,
		},
		{
			name: "working with queued follow-up inputs",
			text: "\n\u2022 Working (3m 53s \u2022 esc to interrupt) \u00b7 1 background terminal running \u00b7 /ps to view \u00b7 /stop to close\n" +
				"\u2022 Queued follow-up inputs\n" +
				"  \u21b3 also fix the other page after this\n" +
				"    arbitrary queued message preview text\n" +
				"    shift + \u2190 edit last queued message\n" +
				"\u203a Write tests for @filename\n" +
				"  gpt-5.5 xhigh fast \u00b7 ~\n",
			want: true,
		},
		{
			name: "queued follow-up status alone is not active codex",
			text: "\n\u2022 Queued follow-up inputs\n" +
				"  \u21b3 also fix the other page after this\n" +
				"    arbitrary queued message preview text\n" +
				"    shift + \u2190 edit last queued message\n" +
				"\u203a Write tests for @filename\n" +
				"  gpt-5.5 xhigh fast \u00b7 ~\n",
			want: false,
		},
		{
			name: "background terminal wait",
			text: "\nWaiting for background terminal\n\n" +
				"\u203a Run tests\n\n" +
				"gpt-5.5 xhigh fast \u00b7 ~\n",
			want: true,
		},
		{
			name: "background terminal wait wrapped with controls",
			text: "\n\u2022 Waiting for background\n" +
				"  terminal \u00b7 1 terminal running \u00b7 /ps to view \u00b7 /stop to close\n\n" +
				"\u203a Run tests\n\n" +
				"gpt-5.6-sol high fast \u00b7 ~\n",
			want: true,
		},
		{
			name: "background terminal wait with running command and queued input",
			text: "\n\u2022 Waiting for background terminal (7m 58s \u2022 esc to interrupt) \u00b7 1 background terminal running \u00b7 /ps to view \u00b7 /stop to close\n" +
				"  \u2514 rgw-ast create analysis/queries/28_primary_neighbour_resolution.sql --expect-absent --stdin <<\u0027SQL\u0027\u2026\n\n" +
				"\u2022 Queued follow-up inputs\n" +
				"  \u21b3 dont forget to add GitHub credential details\n" +
				"    shift + \u2190 edit last queued message\n",
			want: true,
		},
		{
			name: "preparing status with braille spinner and prompt",
			text: "\n\u2022 Preparing audit coverage checks (15m 16s \u2022 esc to int\u2026\n" +
				"\u2801             \u2840          \u2880  \u2808   \u2801 \u2880\n" +
				"\u203a A\n",
			want: true,
		},
		{
			name: "background terminal wait with command detail and braille spinner",
			text: "\nWaiting for background terminal (5m 41s \u2022 esc to inte\u2026\n" +
				"  \u2514 node /tmp/job/capture.mjs\n" +
				"\u2801   \u2808         \u2840\n" +
				"\u203a queued follow-up\n",
			want: true,
		},
		{
			name: "spinner glyph embedded in the codex prompt",
			text: "\n\u2022 Adding findings to HTML (14m 07s \u2022 esc to interrupt) \u2026\n" +
				"\u2801             \u2840          \u2880\n" +
				"\u203a\u2801Ask Codex to do anything\n" +
				"  gpt-6-astra medium \u00b7 ~\n",
			want: true,
		},
		{
			name: "idle codex prompt",
			text: "\n\u203a Run tests\n\n" +
				"gpt-5.5 xhigh fast \u00b7 ~\n",
			want: false,
		},
		{
			name: "transcript mention is not status",
			text: "\nI am working (still) on this now.\n\n" +
				"\u203a next command\n",
			want: false,
		},
		{
			name: "grok build thinking status",
			text: "\nThinking...\n\n" +
				"esc to interrupt\n" +
				"grok-4.3 · ~\n",
			want: true,
		},
		{
			name: "grok prose thinking is not status",
			text: "\ngrok-4.3 · ~\n" +
				"Thinking about the changes...\n" +
				"• Running tests\n" +
				"• Editing file\n",
			want: false,
		},
		{
			name: "grok idle prompt",
			text: "\n> Run tests\n\n" +
				"grok-4.3 · ~\n",
			want: false,
		},
		{
			name: "grok idle navigation keybindings",
			text: "\nTurn completed in 12s.\n\n" +
				"╭─────────────────────────────────────────────────────────────────────────────────────╮\n" +
				"│ ❯ Build anything                                                                    │\n" +
				"╰──────────────────────────────────────────────── Composer 2.5 Fast · always-approve ─╯\n\n" +
				"e:expand  │  Enter:open  │  j/k:nav  │  Shift+l/h:turn  │  Ctrl+e:expand thinking  │\n",
			want: false,
		},
		{
			name: "grok unavailable model prompt is idle",
			text: "\n• Ran git status\n" +
				"  └ ## main...origin/main\n\n" +
				"Model \"grok-build\" is no longer available. Please start a new session.\n\n" +
				"╭─────────────────────────────────────────────────────────────────────────────────────╮\n" +
				"│ ❯                                                                                   │\n" +
				"╰────────────────────────────────────────────── Grok 4.5 (high) · always-approve ─────╯\n",
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := paneTextShowsWorking(test.text); got != test.want {
				t.Fatalf("paneTextShowsWorking() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestClaudePaneTextShowsWorking(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{
			name: "whimsical spinner is working",
			text: "\n✶ Garnishing… (3m 47s · ↓ 17.6k tokens)\n" +
				"⏵⏵ auto mode on (shift+tab to cycle)\n",
			want: true,
		},
		{
			name: "another spinner verb is working",
			text: "\n✶ Whatchamacalliting… (12s · ↓ 1.2k tokens)\n",
			want: true,
		},
		{
			name: "apostrophe spinner verb is working",
			text: "\n✶ Beboppin'… (4s · ↓ 800 tokens)\n",
			want: true,
		},
		{
			name: "waiting for background agents is working",
			text: "\n✻ Waiting for 3 background agents to finish\n" +
				"◯ general-purpose  Searching… 2m 4s · ↓ 85.1k tokens\n" +
				"⏵⏵ auto mode on (shift+tab to cycle)\n",
			want: true,
		},
		{
			name: "subagent row alone is working",
			text: "\n◯ general-purpose  Diffing a… 2m 4s · ↓ 79.8k tokens\n",
			want: true,
		},
		{
			name: "finished claude turn is idle",
			text: "\n✻ Worked for 40s · done Thursday 6:47 PM\n" +
				"⏵⏵ auto mode on (shift+tab to cycle)\n",
			want: false,
		},
		{
			name: "claude transcript bullet is not status",
			text: "\n● Now I'll write the speed-optimized proposal\n" +
				"⏵⏵ auto mode on (shift+tab to cycle)\n",
			want: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := paneTextShowsWorking(test.text); got != test.want {
				t.Fatalf("paneTextShowsWorking() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestClaudeSpinnerGapStaysWorking(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	spinner := "✶ Billowing… (3m 38s · ↓ 16.1k tokens)\n⏵⏵ auto mode on\n"
	missed := "● Typecheck passes. Now local:env: it\n⏵⏵ auto mode on\n"
	done := "✻ Worked for 40s · done Thursday 6:47 PM\n⏵⏵ auto mode on\n"

	working, until := stabilizeClaudeActivity(claudeTailActivity(spinner), now, time.Time{})
	if !working || !until.Equal(now.Add(claudeWorkingHold)) {
		t.Fatalf("spinner = %v %v", working, until)
	}
	working, held := stabilizeClaudeActivity(claudeTailActivity(missed), now.Add(time.Second), until)
	if !working || !held.Equal(until) {
		t.Fatalf("missed frame = %v %v", working, held)
	}
	working, cleared := stabilizeClaudeActivity(claudeTailActivity(done), now.Add(2*time.Second), until)
	if working || !cleared.IsZero() {
		t.Fatalf("done = %v %v", working, cleared)
	}
	working, _ = stabilizeClaudeActivity(claudeTailActivity(missed), until.Add(time.Second), until)
	if working {
		t.Fatal("expired hold still counted as working")
	}
}

func TestIsClaudeProcess(t *testing.T) {
	if !isClaudeProcess(ProcessInfo{Comm: "claude", Args: "/home/ryan/.local/bin/claude"}) {
		t.Fatal("claude comm was not recognized")
	}
	if !isClaudeProcess(ProcessInfo{Comm: "node", Args: "/usr/bin/claude"}) {
		t.Fatal("claude path was not recognized")
	}
	if isClaudeProcess(ProcessInfo{Comm: "codex", Args: "codex"}) {
		t.Fatal("codex was treated as claude")
	}
}

func TestGrokPaneTextShowsWorking(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{
			name: "grok tool status bar with stop",
			text: "    ⠸ Run tmux capture-pane -p -t %26 -S -50 > /tmp/grok… 0.0s 20s ⇣105k [↓][stop]\n",
			want: true,
		},
		{
			name: "grok monitor still running status",
			text: "\nWorked for 54s\n\n◎ 1 monitor still running\n",
			want: true,
		},
		{
			name: "grok monitor status across animation glyphs",
			text: "\n◎ 1 monitor still running\n◉ 2 monitors still running\n○ 1 monitor still running\n",
			want: true,
		},
		{
			name: "grok compound monitor still running",
			text: "\n◎ 1 command · 2 monitors · 1 loop · 1 subagent still running\n",
			want: true,
		},
		{
			name: "grok watchers monitor row",
			text: "\nWorked for 4m35s\n\n▾ Watchers 1\n⁙ Monitor Watch manual DEV release                             7m27s\n",
			want: true,
		},
		{
			name: "grok monitor row across spinner frames",
			text: "\n⋅ Monitor tail the log\n: Monitor tail the log\n⸬ Monitor tail the log\n",
			want: true,
		},
		{
			name: "grok collapsed watchers header",
			text: "\nWorked for 1m\n\n▸ Watchers 2\n",
			want: true,
		},
		{
			name: "grok tasks run row",
			text: "\nWorked for 44s\n\n▾ Tasks 1\n⋅ Run Start Claude on model state                             33m57s\n",
			want: true,
		},
		{
			name: "grok run row across spinner frames",
			text: "\n⋅ Run go test\n: Run go test\n⸬ Run go test\n⁙ Run go test\n",
			want: true,
		},
		{
			name: "grok collapsed tasks header",
			text: "\nWorked for 1m\n\n▸ Tasks 1\n",
			want: true,
		},
		{
			name: "grok worked and recap is idle",
			text: "\n    ⠸ Run tmux capture-pane [↓][stop]\n" +
				"▾ Tasks 1\n" +
				"⋅ Run the signup again\n" +
				"Worked for 2m\n" +
				"Recap: We ran the signup and left the ticket for QA.\n" +
				"\n❯ next\n",
			want: false,
		},
		{
			name: "grok recap dash is idle",
			text: "\nWorked for 1m\nRecap — We fixed the matcher.\n",
			want: false,
		},
		{
			name: "grok monitor after the recap is working",
			text: "\nWorked for 1m\nRecap: We left a monitor on the release.\n▾ Watchers 1\n⁙ Monitor Watch the release 2m\n",
			want: true,
		},
		{
			name: "grok prose about a run is idle",
			text: "\nI will run the tests after this turn.\n\n❯ next prompt\n",
			want: false,
		},
		{
			name: "grok prose about watchers is idle",
			text: "\nThe Watchers panel lists a monitor while we wait.\n\n❯ next prompt\n",
			want: false,
		},
		{
			name: "grok prose about monitor is not status",
			text: "\nThe monitor is still running while we wait for news.\n\n❯ next prompt\n",
			want: false,
		},
		{
			name: "grok waiting for response",
			text: "    \u283c Waiting for response\u2026 1.2s                                             3m \u21e3100k\n",
			want: true,
		},
		{
			name: "grok waiting on subagent",
			text: "  \u28bf Waiting on subagent\u2026 4s\n",
			want: true,
		},
		{
			name: "grok waiting phrase in transcript prose is idle",
			text: "\nWaiting for response in the transcript is not a status row.\n\n❯ next\n",
			want: false,
		},
		{
			name: "grok turn completed idle",
			text: "Turn completed in 32s.\n\n❯ lets see\n",
			want: false,
		},
		{
			name: "grok thinking header",
			text: "  ┃  ◆ Thinking…\n    ⠸ Thinking… 1.5s                                             9.0s ⇣104k [stop]\n",
			want: true,
		},
		{
			name: "grok unavailable model prompt",
			text: "\n• Ran git status\n" +
				"  └ ## main...origin/main\n\n" +
				"It stops people from rushing into execution with fuzzy thinking.\n\n" +
				"Model \"grok-build\" is no longer available. Please start a new session.\n\n" +
				"╭─────────────────────────────────────────────────────────────────────────────────────╮\n" +
				"│ ❯                                                                                   │\n" +
				"╰────────────────────────────────────────────── Grok 4.5 (high) · always-approve ─────╯\n",
			want: false,
		},
		{
			name: "grok idle navigation keybindings",
			text: "Turn completed in 12s.\n\n" +
				"e:expand  │  Enter:open  │  j/k:nav  │  Shift+l/h:turn  │  Ctrl+e:expand thinking  │\n",
			want: false,
		},
		{
			name: "grok foreground row with down marker",
			text: "    ⠸ Capture live status lines… 0.1s                                             3m55s ⇣203k [↓]\n",
			want: true,
		},
		{
			name: "grok dispatch run and monitor rows",
			text: "\nWorked for 1m\n\n▾ Tasks 1\n⋅ Dispatch watch the QA deploy                             2m\n⠦ Run go test\n⠧ Monitor the log\n",
			want: true,
		},
		{
			name: "grok running dispatch title row",
			text: "\n⠧ Running: Dispatch|dispatch\n",
			want: true,
		},
		{
			name: "grok foreground row above the recap is idle",
			text: "\n    ⠸ Capture live status lines… 0.1s [↓]\nWorked for 2m\nRecap — We left the monitor.\n\n❯ next\n",
			want: false,
		},
		{
			name: "grok transcript dispatch is idle",
			text: "\n◆ Task started: Dispatch and watch the QA deploy of the split\n" +
				"◆ Task completed\n" +
				"Worked for 4m\n" +
				"Recap — The deploy finished.\n\n❯ next\n",
			want: false,
		},
		{
			name: "grok prose dispatch run monitor is idle",
			text: "\nI will dispatch a run and leave a monitor on the log.\n\nThe title was Running: Dispatch|dispatch while the turn sat idle.\n\n❯ next\n",
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := grokPaneTextShowsWorking(test.text); got != test.want {
				t.Fatalf("grokPaneTextShowsWorking() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestGrokTitleShowsWorking(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  bool
	}{
		{name: "dispatch title", title: "⠧ - Running: Dispatch|dispatch - Pass app - grok", want: true},
		{name: "run title", title: "⠸ - Running: Run|run - session - grok", want: true},
		{name: "monitor title", title: "⠦ - Running: Monitor|monitor - session - grok", want: true},
		{name: "thinking title", title: "⠼ - Thinking - session - grok", want: true},
		{name: "idle title", title: "Stream 14 unified local ticket batch - grok", want: false},
		{name: "idle title mentioning dispatch", title: "Running: Dispatch|dispatch notes - grok", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := grokTitleShowsWorking(test.title); got != test.want {
				t.Fatalf("grokTitleShowsWorking(%q) = %v, want %v", test.title, got, test.want)
			}
		})
	}
}

func TestPaneCodexProfileReadsExplicitProfile(t *testing.T) {
	tests := []struct {
		name string
		args string
		want string
	}{
		{name: "space separated", args: "codex --profile zai", want: "zai"},
		{name: "equals separated", args: "/usr/bin/codex --profile=zai resume", want: "zai"},
		{name: "default codex", args: "codex", want: ""},
		{name: "profile without value", args: "codex --profile", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := codexProfileFromArgs(test.args); got != test.want {
				t.Fatalf("codexProfileFromArgs(%q) = %q, want %q", test.args, got, test.want)
			}
		})
	}
}

func TestPaneCodexProfileFollowsPaneProcessTree(t *testing.T) {
	api := NewAPI(nil)
	pane := Pane{PanePID: 10}
	processes := map[int]ProcessInfo{
		9:  {PID: 9, PPID: 1, Comm: "bash", Args: "bash"},
		10: {PID: 10, PPID: 9, Comm: "bash", Args: "bash"},
		11: {PID: 11, PPID: 10, Comm: "codex", Args: "codex --profile zai"},
	}
	children := map[int][]int{9: {10}, 10: {11}}

	if got := api.PaneCodexProfile(pane, processes, children); got != "zai" {
		t.Fatalf("PaneCodexProfile() = %q, want zai", got)
	}
}

func TestGrokSessionIDFollowsWorkspaceDirectory(t *testing.T) {
	homeID := "01a17622-5fd9-7bc2-81ff-3e200a8c16b7"
	wiomID := "01a16844-3a1f-7331-a0f7-73684c4ac73d"
	adeID := "01a0f595-6592-7423-ad1f-fa8dbb505b5c"
	output := strings.Join([]string{
		"/home/ryan/.grok/sessions/%2Fhome%2Fryan/" + homeID + "/terminal/call-73646990-5b12-4bb8-ade8-512c53f58b46-23.log",
		"/home/ryan/.grok/sessions/%2Fhome%2Fryan/" + homeID + "/events.jsonl",
		"/home/ryan/.grok/sessions/%2Fhome%2Fryan%2FWork%2Fwiom/" + wiomID + "/updates.jsonl",
	}, "\n")
	hits := grokSessionHitsFromOutput(output)

	id, workspace := chooseGrokSession(hits, "/home/ryan/Work/wiom")
	if id != wiomID || workspace != "/home/ryan/Work/wiom" {
		t.Fatalf("wiom session = %s in %s", id, workspace)
	}
	id, workspace = chooseGrokSession(hits, "/home/ryan")
	if id != homeID || workspace != "/home/ryan" {
		t.Fatalf("home session = %s in %s", id, workspace)
	}
	id, workspace = chooseGrokSession(hits, "/tmp")
	if id != homeID || workspace != "/home/ryan" {
		t.Fatalf("unmatched pane session = %s in %s", id, workspace)
	}

	ade := "/home/ryan/.grok/sessions/%2Fhome%2Fryan%2FWork%2Famerican_dream_enterprises/" + adeID + "/events.jsonl"
	id, workspace = chooseGrokSession(grokSessionHitsFromOutput(ade), "/home/ryan")
	if id != adeID || workspace != "/home/ryan/Work/american_dream_enterprises" {
		t.Fatalf("ade session = %s in %s", id, workspace)
	}
}

func TestGrokSessionIDIgnoresCodexRolloutPaths(t *testing.T) {
	output := "/home/ryan/.codex/sessions/2026/10/26/rollout-2026-10-26T08-00-00-01a17622-5fd9-7bc2-81ff-3e200a8c16b7.jsonl"
	if hits := grokSessionHitsFromOutput(output); len(hits) != 0 {
		t.Fatalf("codex rollout parsed as grok: %#v", hits)
	}
	if id := rolloutThreadIDFromOutput(output); id != "01a17622-5fd9-7bc2-81ff-3e200a8c16b7" {
		t.Fatalf("rolloutThreadIDFromOutput() = %q", id)
	}
}

func TestGrokSessionIDFromArgs(t *testing.T) {
	id := "01a17622-5fd9-7bc2-81ff-3e200a8c16b7"
	tests := []struct {
		args string
		want string
	}{
		{args: "grok --resume " + id, want: id},
		{args: "grok --resume=" + id, want: id},
		{args: "grok -r " + id, want: id},
		{args: "grok --resume latest-home-session", want: ""},
		{args: "grok", want: ""},
	}
	for _, test := range tests {
		if got := grokSessionIDFromArgs(test.args); got != test.want {
			t.Fatalf("grokSessionIDFromArgs(%q) = %q, want %q", test.args, got, test.want)
		}
	}
}

func TestMatchActiveGrokSessionKeepsWorkspace(t *testing.T) {
	homeID := "01a17622-5fd9-7bc2-81ff-3e200a8c16b7"
	wiomID := "01a16844-3a1f-7331-a0f7-73684c4ac73d"
	rows := []activeGrokSession{
		{SessionID: homeID, PID: 10, PIDStart: 100, CWD: "/home/ryan"},
		{SessionID: wiomID, PID: 11, PIDStart: 200, CWD: "/home/ryan/Work/wiom"},
	}
	id, cwd, ok := matchActiveGrokSession(rows, 11, 200, true)
	if !ok || id != wiomID || cwd != "/home/ryan/Work/wiom" {
		t.Fatalf("match = %v %s %s", ok, id, cwd)
	}
	if _, _, ok := matchActiveGrokSession(rows, 11, 999, true); ok {
		t.Fatal("reused pid matched a stale grok session")
	}
	id, cwd, ok = matchActiveGrokSession(rows, 10, 0, false)
	if !ok || id != homeID || cwd != "/home/ryan" {
		t.Fatalf("unchecked start = %v %s %s", ok, id, cwd)
	}
}

func TestGrokSessionPrefersParentOverSubagentInSameWorkspace(t *testing.T) {
	parentID := "01a17622-5fd9-7bc2-81ff-3e200a8c16b7"
	subID := "01a17622-aaaa-7bc2-81ff-3e200a8c16b7"
	base := "/home/ryan/.grok/sessions/%2Fhome%2Fryan/"
	output := strings.Join([]string{
		base + subID + "/events.jsonl",
		base + parentID + "/terminal/call-73646990-5b12-4bb8-ade8-512c53f58b46-23.log",
		base + parentID + "/events.jsonl",
	}, "\n")
	id, workspace := chooseGrokSession(grokSessionHitsFromOutput(output), "/home/ryan")
	if id != parentID || workspace != "/home/ryan" {
		t.Fatalf("session = %s in %s", id, workspace)
	}
}
