package tui

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"better-scaffold/internal/config"
	"better-scaffold/internal/generate"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

var accent = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true)
var muted = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func form(groups ...*huh.Group) error {
	return huh.NewForm(groups...).WithTheme(huh.ThemeDracula()).WithShowHelp(true).Run()
}

func Run(ctx context.Context, c config.Config, dryRun bool) error {
	if c.Secrets == nil {
		c.Secrets = map[string]string{}
	}
	fmt.Println(accent.Render("BETTER SCAFFOLD"))
	fmt.Println(muted.Render("Keep your momentum. Start with a connected Next.js app."))
	payments, ai := c.Payments != "none", c.AI != "none"
	for {
		err := form(
			huh.NewGroup(
				huh.NewInput().Title("Project name").Description("Lowercase letters, numbers, and hyphens.").Value(&c.Name).Validate(func(s string) error {
					if !validName.MatchString(s) {
						return fmt.Errorf("use a lowercase project name, such as my-app")
					}
					return nil
				}),
				huh.NewInput().Title("Destination").Description("Leave blank to use the project name. Existing files are never overwritten.").Value(&c.Directory),
				huh.NewSelect[string]().Title("Package manager").Options(huh.NewOption("Bun", "bun"), huh.NewOption("pnpm", "pnpm"), huh.NewOption("npm", "npm")).Value(&c.PackageManager),
			).Title("01  Project"),
			huh.NewGroup(
				huh.NewMultiSelect[string]().Title("Social sign-in").Description("Email and password are always included.").Options(huh.NewOption("Google", "google"), huh.NewOption("GitHub", "github")).Value(&c.OAuth),
				huh.NewConfirm().Title("Include payments?").Value(&payments),
				huh.NewConfirm().Title("Include the Vercel AI SDK?").Value(&ai),
			).Title("02  Batteries"),
			huh.NewGroup(
				huh.NewConfirm().Title("Motion animations").Value(&c.Motion),
				huh.NewConfirm().Title("Elysia API with Eden client").Value(&c.Elysia),
				huh.NewConfirm().Title("TanStack Query").Value(&c.Query),
				huh.NewConfirm().Title("Zod application validation").Value(&c.Zod),
				huh.NewInput().Title("shadcn preset").Description("Paste a preset code or URL from ui.shadcn.com/create, or leave blank for defaults.").Value(&c.Preset),
			).Title("03  Options"),
		)
		if err != nil {
			return err
		}
		if c.Directory == "" {
			c.Directory = c.Name
		}
		var providerFields []huh.Field
		if payments {
			if c.Payments == "none" {
				c.Payments = "polar"
			}
			providerFields = append(providerFields, huh.NewSelect[string]().Title("Payment provider").Options(huh.NewOption("Polar", "polar"), huh.NewOption("Stripe", "stripe")).Value(&c.Payments))
		} else {
			c.Payments = "none"
		}
		if ai {
			if c.AI == "none" {
				c.AI = "openai"
			}
			providerFields = append(providerFields, huh.NewSelect[string]().Title("AI provider").Options(huh.NewOption("OpenAI", "openai"), huh.NewOption("Anthropic", "anthropic")).Value(&c.AI))
		} else {
			c.AI = "none"
		}
		if len(providerFields) > 0 {
			if err := form(huh.NewGroup(providerFields...).Title("04  Providers")); err != nil {
				return err
			}
		}
		keys := credentialKeys(c)
		values := make([]string, len(keys))
		fields := make([]huh.Field, 0, len(keys))
		for i, key := range keys {
			values[i] = c.Secrets[key]
			fields = append(fields, huh.NewInput().Title(key).Description("Optional. Leave blank and configure .env later.").EchoMode(huh.EchoModePassword).Value(&values[i]))
		}
		if err := form(huh.NewGroup(fields...).Title("05  Credentials")); err != nil {
			return err
		}
		c.Secrets = map[string]string{}
		for i, key := range keys {
			c.Secrets[key] = values[i]
		}
		action := "generate"
		if dryRun {
			action = "preview"
		}
		label := "Generate project"
		if dryRun {
			label = "Finish preview without writing files"
		}
		if err := form(huh.NewGroup(
			huh.NewNote().Title("06  Review").Description(Summary(c)),
			huh.NewSelect[string]().Title("Ready?").Options(huh.NewOption(label, action), huh.NewOption("Edit selections", "edit"), huh.NewOption("Cancel", "cancel")).Value(&action),
		)); err != nil {
			return err
		}
		if action == "cancel" {
			return nil
		}
		if action == "edit" {
			continue
		}
		if err := c.Validate(); err != nil {
			return err
		}
		if dryRun {
			fmt.Println(Summary(c))
			fmt.Println("Preview complete. No files were written.")
			return nil
		}
		return runGeneration(ctx, c)
	}
}

func credentialKeys(c config.Config) []string {
	keys := []string{"DATABASE_URL"}
	for _, p := range c.OAuth {
		if p == "google" {
			keys = append(keys, "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET")
		} else if p == "github" {
			keys = append(keys, "GITHUB_CLIENT_ID", "GITHUB_CLIENT_SECRET")
		}
	}
	switch c.Payments {
	case "polar":
		keys = append(keys, "POLAR_ACCESS_TOKEN", "POLAR_WEBHOOK_SECRET", "POLAR_PRODUCT_ID")
	case "stripe":
		keys = append(keys, "STRIPE_SECRET_KEY", "STRIPE_WEBHOOK_SECRET", "STRIPE_PRICE_ID")
	}
	switch c.AI {
	case "openai":
		keys = append(keys, "OPENAI_API_KEY")
	case "anthropic":
		keys = append(keys, "ANTHROPIC_API_KEY")
	}
	return keys
}

func Summary(c config.Config) string {
	social := "Email and password"
	if len(c.OAuth) > 0 {
		social += ", " + strings.Join(c.OAuth, ", ")
	}
	extras := []string{}
	if c.Motion {
		extras = append(extras, "Motion")
	}
	if c.Elysia {
		extras = append(extras, "Elysia + Eden")
	}
	if c.Query {
		extras = append(extras, "TanStack Query")
	}
	if c.Zod {
		extras = append(extras, "Zod")
	}
	if len(extras) == 0 {
		extras = append(extras, "none")
	}
	preset := c.Preset
	if preset == "" {
		preset = "default"
	}
	migration := "Deferred. Add DATABASE_URL to .env, then run db:migrate."
	if c.Secrets["DATABASE_URL"] != "" {
		migration = "Will run against your supplied database after generation."
	}
	missing := 0
	for _, key := range credentialKeys(c) {
		if c.Secrets[key] == "" {
			missing++
		}
	}
	return fmt.Sprintf("Project: %s\nDestination: %s\nPackage manager: %s\n\nNext.js · TypeScript · Tailwind · shadcn · Geist\nPostgreSQL · Drizzle · Better Auth\nAuthentication: %s\nPayments: %s\nAI: %s\nOptional: %s\nshadcn preset: %s\n\nMigration: %s\nCredentials left for later: %d\nSecrets are stored only in the gitignored .env.", c.Name, c.Directory, c.PackageManager, social, c.Payments, c.AI, strings.Join(extras, ", "), preset, migration, missing)
}

type progressMsg string
type finishedMsg struct{ err error }
type tickMsg struct{}
type progressModel struct {
	ctx          context.Context
	cancel       context.CancelFunc
	config       config.Config
	events       chan string
	lines        []string
	frame, width int
	stopping     bool
	err          error
}

func (m progressModel) Init() tea.Cmd { return tea.Batch(m.listen(), m.start(), tick()) }
func (m progressModel) start() tea.Cmd {
	return func() tea.Msg {
		err := generate.Run(m.ctx, m.config, func(s string) {
			select {
			case m.events <- s:
			case <-m.ctx.Done():
			}
		})
		return finishedMsg{err}
	}
}
func (m progressModel) listen() tea.Cmd {
	return func() tea.Msg {
		select {
		case s := <-m.events:
			return progressMsg(s)
		case <-m.ctx.Done():
			return nil
		}
	}
}
func tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}
func (m progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "esc" {
			m.stopping = true
			m.cancel()
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tickMsg:
		m.frame++
		return m, tick()
	case progressMsg:
		m.lines = append(m.lines, string(msg))
		return m, m.listen()
	case finishedMsg:
		m.err = msg.err
		return m, tea.Quit
	}
	return m, nil
}
func (m progressModel) View() string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	heading := "Creating " + m.config.Name
	if m.stopping {
		heading = "Stopping generation…"
	}
	s := accent.Render(frames[m.frame%len(frames)]+"  "+heading) + "\n\n"
	start := 0
	if len(m.lines) > 8 {
		start = len(m.lines) - 8
	}
	for _, line := range m.lines[start:] {
		s += "  " + line + "\n"
	}
	s += "\n" + muted.Render("ctrl+c to cancel · generated files are retained")
	width := m.width - 4
	if width < 20 {
		width = 20
	}
	if width > 96 {
		width = 96
	}
	return lipgloss.NewStyle().Width(width).Padding(1, 2).Render(s)
}
func runGeneration(ctx context.Context, c config.Config) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	m := progressModel{ctx: ctx, cancel: cancel, config: c, events: make(chan string, 64), width: 80}
	result, err := tea.NewProgram(m).Run()
	if err != nil {
		return err
	}
	final, ok := result.(progressModel)
	if !ok {
		return fmt.Errorf("generation ended unexpectedly")
	}
	if final.err != nil {
		return fmt.Errorf("generation stopped: %w\nGenerated files remain in %s. Inspect the failed step before retrying its command", final.err, c.Directory)
	}
	fmt.Println(accent.Render("Your app is ready."))
	fmt.Printf("\n  cd %q\n  %s run dev\n\n", c.Directory, c.PackageManager)
	if c.Secrets["DATABASE_URL"] == "" {
		fmt.Printf("Add DATABASE_URL to .env and run %s run db:migrate before signing in.\n", c.PackageManager)
	}
	fmt.Println("See the generated README for provider callbacks, webhooks, and remaining credentials.")
	return nil
}
