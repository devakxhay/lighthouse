package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devakxhay/lighthouse/internal/bins"
)

type installStartMsg struct {
	index int
}

type installLogMsg struct {
	text string
}

type installDoneMsg struct {
	index  int
	status bins.Status
	err    error
}

type allDoneMsg struct{}

type Model struct {
	db        bins.DB
	specs     []bins.Installer
	checked   map[int]bool
	cursor    int
	installing bool
	activeIdx int
	logs      []string
	statuses  map[int]bins.Status
	errs      map[int]error
	done      bool
	ctx       context.Context
	cancel    context.CancelFunc
	program   *tea.Program
}

func NewModel(db bins.DB, specs []bins.Installer) *Model {
	ctx, cancel := context.WithCancel(context.Background())
	checked := make(map[int]bool)
	statuses := make(map[int]bins.Status)

	for i, spec := range specs {
		status := spec.Detect()
		statuses[i] = status
		// Pre-check the ones that are already installed, as requested
		if status.Found {
			checked[i] = true
		} else {
			checked[i] = false
		}
	}

	return &Model{
		db:       db,
		specs:    specs,
		checked:  checked,
		statuses: statuses,
		errs:     make(map[int]error),
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
}

func (m *Model) startInstallation() {
	m.installing = true
	go func() {
		for i, spec := range m.specs {
			if !m.checked[i] {
				continue
			}

			// Send message to start installing this spec
			m.program.Send(installStartMsg{index: i})

			status, err := spec.Install(m.ctx, func(msg string) {
				m.program.Send(installLogMsg{text: msg})
			})

			if err == nil {
				// Save override in DB
				err = m.db.SetRuntimeOverride(spec.Name(), status.BinPath)
			}

			// Send message that this installer is done
			m.program.Send(installDoneMsg{index: i, status: status, err: err})
		}
		// Send final all done message
		m.program.Send(allDoneMsg{})
	}()
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.cancel()
			return m, tea.Quit

		case "up", "k":
			if !m.installing && !m.done {
				m.cursor--
				if m.cursor < 0 {
					m.cursor = len(m.specs) - 1
				}
			}

		case "down", "j":
			if !m.installing && !m.done {
				m.cursor++
				if m.cursor >= len(m.specs) {
					m.cursor = 0
				}
			}

		case " ":
			if !m.installing && !m.done {
				m.checked[m.cursor] = !m.checked[m.cursor]
			}

		case "enter":
			if !m.installing && !m.done {
				// Count checked items
				hasChecked := false
				for _, check := range m.checked {
					if check {
						hasChecked = true
						break
					}
				}
				if hasChecked {
					m.startInstallation()
				}
			} else if m.done {
				return m, tea.Quit
			}
		}

	case installStartMsg:
		m.activeIdx = msg.index
		m.logs = []string{}

	case installLogMsg:
		// Keep last 15 lines of log output
		m.logs = append(m.logs, msg.text)
		if len(m.logs) > 15 {
			m.logs = m.logs[len(m.logs)-15:]
		}

	case installDoneMsg:
		m.statuses[msg.index] = msg.status
		if msg.err != nil {
			m.errs[msg.index] = msg.err
		}

	case allDoneMsg:
		m.done = true
		m.installing = false
	}

	return m, nil
}

func (m *Model) View() string {
	var sb strings.Builder

	sb.WriteString("\x1b[1;36m🔦 Lighthouse - Binary Dependency Installer\x1b[0m\n\n")

	if !m.installing && !m.done {
		sb.WriteString("Select runtimes to install/update:\n\n")

		for i, spec := range m.specs {
			cursorStr := "  "
			if m.cursor == i {
				cursorStr = "\x1b[1;36m> \x1b[0m"
			}

			checkStr := "[ ]"
			if m.checked[i] {
				checkStr = "\x1b[1;32m[✓]\x1b[0m"
			}

			statusStr := ""
			status := m.statuses[i]
			if status.Found {
				statusStr = fmt.Sprintf(" \x1b[2;33m(already installed: %s)\x1b[0m", status.BinPath)
			} else {
				statusStr = " \x1b[2;31m(not found)\x1b[0m"
			}

			sb.WriteString(fmt.Sprintf("%s%s %-30s%s\n", cursorStr, checkStr, spec.Label(), statusStr))
		}

		sb.WriteString("\n\x1b[2mSpace to toggle · Enter to start install · q to quit\x1b[0m\n")
		return sb.String()
	}

	if m.installing {
		activeSpec := m.specs[m.activeIdx]
		sb.WriteString(fmt.Sprintf("Installing \x1b[1;33m%s\x1b[0m...\n", activeSpec.Label()))
		sb.WriteString("──────────────────────────────────────────────────\n")
		for _, logLine := range m.logs {
			sb.WriteString(fmt.Sprintf("  \x1b[2m%s\x1b[0m\n", logLine))
		}
		sb.WriteString("──────────────────────────────────────────────────\n")
		return sb.String()
	}

	if m.done {
		sb.WriteString("Installation Complete!\n\n")
		sb.WriteString("Status Summary:\n")

		for i, spec := range m.specs {
			if !m.checked[i] {
				sb.WriteString(fmt.Sprintf("  %-30s \x1b[2;37m(skipped)\x1b[0m\n", spec.Label()))
				continue
			}

			if err := m.errs[i]; err != nil {
				sb.WriteString(fmt.Sprintf("  %-30s \x1b[1;31m✗ failed: %v\x1b[0m\n", spec.Label(), err))
			} else {
				status := m.statuses[i]
				sb.WriteString(fmt.Sprintf("  %-30s \x1b[1;32m✓ installed: %s\x1b[0m\n", spec.Label(), status.BinPath))
			}
		}

		sb.WriteString("\n\x1b[1mRestart Lighthouse service to apply changes:\x1b[0m\n")
		sb.WriteString("  \x1b[36msudo systemctl restart lighthouse\x1b[0m\n\n")
		sb.WriteString("\x1b[2mPress Enter or q to exit\x1b[0m\n")
	}

	return sb.String()
}
