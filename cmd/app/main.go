// Provides a simple TUI to view and manipulate register contents based
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rwirdemann/modbusmanager"
	"io"
	"log"
	"log/slog"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	stateRegisterList = iota
	stateRegisterInput
)

var (
	configPath *string // base directory of config files
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type model struct {
	state         int
	table         table.Model
	modbusModel   modbusModel
	propertyTable table.Model
	currentRow    row
	registerInput textinput.Model
}

func newModel() model {
	tableModel, err := newGatewayModel()
	if err != nil {
		log.Fatal(err)
	}

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(true)

	// create register table
	columns := []table.Column{
		{Title: "Device", Width: 14},
		{Title: "Slave Adr", Width: 10},
		{Title: "Address", Width: 8},
		{Title: "Action", Width: 6},
		{Title: "Datatype", Width: 10},
		{Title: "Type", Width: 10},
		{Title: "Value", Width: 20},
	}

	rows := tableModel.tableRows()
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(len(rows)+1),
	)
	t.SetStyles(s)

	// create property table
	propertyColumns := []table.Column{
		{Title: "Name", Width: 30},
		{Title: "Processing", Width: 10},
		{Title: "Data Type", Width: 13},
		{Title: "Value", Width: 14},
	}
	propertyTable := table.New(
		table.WithColumns(propertyColumns),
		table.WithHeight(len(rows)+1),
	)
	s.Selected = s.Selected.
		Foreground(lipgloss.NoColor{}).
		Background(lipgloss.NoColor{}).
		Bold(false)
	propertyTable.SetStyles(s)
	return model{table: t, modbusModel: tableModel, registerInput: textinput.New(), state: stateRegisterList, propertyTable: propertyTable}
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*1, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Init() tea.Cmd { return tickCmd() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmds []tea.Cmd
		cmd  tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case stateRegisterList:
			switch msg.String() {
			case "esc":
				if m.table.Focused() {
					m.table.Blur()
				} else {
					m.table.Focus()
				}
			case "q", "ctrl+c":
				return m, tea.Quit
			case "enter":
				m.currentRow = m.modbusModel.rows[m.table.Cursor()]
				m.registerInput.SetValue(fmt.Sprintf("%v", m.currentRow.register.RawData))
				m.registerInput.SetCursor(len(m.registerInput.Value()))
				m.registerInput.Focus()
				m.table.Blur()
				m.state = stateRegisterInput
			}
		case stateRegisterInput:
			switch msg.String() {
			case "esc":
				m.table.Focus()
				m.state = stateRegisterList
			case "enter":
				switch m.currentRow.register.RegisterType {
				case "discrete":
					m.currentRow.register.Datatype = "BOOL"
					m.currentRow.register.RawData = toBool(m.registerInput.Value())
				case "holding", "input":
					switch m.currentRow.register.Datatype {
					case "T64T1234":
						m.currentRow.register.RawData = toUnt64(m.registerInput.Value())
					case "F32T1234":
						m.currentRow.register.RawData = toFloat32(m.registerInput.Value())
					}
				}
				err := modbusmanager.WriteRegister(m.currentRow.register)
				if err != nil {
					slog.Error(err.Error())
				}
				m.table.Focus()
				m.state = stateRegisterList
			}
		}
	case tickMsg:
		m.modbusModel, _ = m.modbusModel.update()
		cmds = append(cmds, tickCmd())
	}

	m.table, cmd = m.table.Update(msg)
	cmds = append(cmds, cmd)

	m.registerInput, cmd = m.registerInput.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	rows := m.modbusModel.tableRows()
	m.table.SetRows(rows)
	s := m.renderRegisterTable()
	if m.state == stateRegisterInput {
		s = lipgloss.JoinHorizontal(lipgloss.Top, s, m.renderRegisterForm())
	}
	//s = lipgloss.JoinHorizontal(lipgloss.Top, s, m.renderPropertyTable())
	return s
}

func (m model) renderRegisterTable() string {
	return baseStyle.Render(m.table.View()) + "\n  " + m.table.HelpView() + " • <enter> update register value" + "\n"
}

func (m model) renderPropertyTable() string {
	return baseStyle.Render(m.propertyTable.View())
}

func (m model) renderRegisterForm() string {
	s := fmt.Sprintf("Address: 0x%X\n\n", m.currentRow.register.Address)
	s += m.registerInput.View() + "\n\n"
	s += "enter - save • esc - discard"
	return baseStyle.Render(s)
}

func init() {
	slog.Info("Initializing main")
}

var config modbusmanager.Config

func main() {
	configPath = flag.String("config", "config", "config base directory")
	flag.Parse()

	bb, err := os.ReadFile(path.Join(*configPath, "modbus.json"))
	if err != nil {
		log.Fatal(err)
	}
	if err := json.NewDecoder(bytes.NewReader(bb)).Decode(&config); err != nil {
		log.Fatal(err)
	}
	modbusmanager.Init(config)

	m := newModel()
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

func toFloat32(s string) float32 {
	f, err := strconv.ParseFloat(s, 32)
	if err != nil {
		slog.Error(err.Error())
		return 0
	}
	return float32(f)
}

func toBool(s string) bool {
	b, err := strconv.ParseBool(s)
	if err != nil {
		slog.Error(err.Error())
		return false
	}
	return b
}

func toUnt64(s string) uint64 {
	i, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		slog.Error(err.Error())
		return 0
	}
	return i
}

type modbusModel struct {
	slaves []slave
	rows   []row
}

type slave struct {
	name     string
	discrete []modbusmanager.Register
	input    []modbusmanager.Register
	holding  []modbusmanager.Register
}

type row struct {
	device       string
	slaveAddress string
	register     modbusmanager.Register
}

func newGatewayModel() (modbusModel, error) {
	m := modbusModel{}
	m.slaves = readConfig()
	return m.update()
}

func (m modbusModel) update() (modbusModel, error) {
	m.rows = nil
	for _, s := range m.slaves {
		discrete, err := modbusmanager.ReadDiscrete(s.discrete)
		if err != nil {
			return m, err
		}
		m.rows = append(m.rows, toRows(s, discrete)...)

		input, err := modbusmanager.ReadInput(s.input, 3)
		if err != nil {
			return m, err
		}
		m.rows = append(m.rows, toRows(s, input)...)

		holding, err := modbusmanager.ReadHolding(s.holding, 3)
		if err != nil {
			return m, err
		}
		m.rows = append(m.rows, toRows(s, holding)...)
	}

	return m, nil
}

func toRows(s slave, registers []modbusmanager.Register) []row {
	var rows []row
	for _, r := range registers {
		rows = append(rows, row{
			device:       s.name,
			slaveAddress: fmt.Sprintf("%d", r.SlaveAddress),
			register:     r,
		})
	}
	return rows
}

func (m modbusModel) tableRows() []table.Row {
	var rows []table.Row
	for _, r := range m.rows {
		rows = append(rows, table.Row{
			r.device,
			r.slaveAddress,
			fmt.Sprintf("0x%X", r.register.Address),
			r.register.Action,
			r.register.Datatype,
			r.register.RegisterType,
			fmt.Sprintf("%v", r.register.RawData),
		})
	}
	return rows
}

func readConfig() []slave {
	var slaves []slave
	for _, serial := range config.Serial {
		for _, s := range serial.Slaves {
			var slave slave
			slave.name = s.HardwareMaker
			r := readFile(path.Join(*configPath, s.HardwareMaker, "register.dsl"))
			registers, err := parseRegisterDSL(r, s.Address)
			if err != nil {
				log.Fatal(err)
			}

			for _, register := range registers {
				switch register.RegisterType {
				case "discrete":
					slave.discrete = append(slave.discrete, register)
				case "holding":
					slave.holding = append(slave.holding, register)
				case "input":
					slave.input = append(slave.input, register)
				}
			}
			slaves = append(slaves, slave)
		}
	}
	return slaves
}

func readFile(name string) io.Reader {
	bb, err := os.ReadFile(name)
	if err != nil {
		log.Fatal(err)
	}
	return bytes.NewReader(bb)
}

func parseRegisterDSL(reader io.Reader, slaveAddress uint8) ([]modbusmanager.Register, error) {
	dsl := readDSL(reader)
	var registers []modbusmanager.Register

	for _, l := range dsl {
		line := strings.Trim(l, " ")
		if !strings.HasPrefix(line, "read") && !strings.HasPrefix(line, "write") {
			return nil, fmt.Errorf("register.dsl: statement '%s' doesn't start with 'read' or 'write'", line)
		}
		ff := strings.Fields(line)
		if len(ff) != 6 {
			return nil, fmt.Errorf("register.dsl: statement '%s' contains invalid keywords", line)
		}
		reg := modbusmanager.Register{
			SlaveAddress: slaveAddress,
			Action:       ff[0],
			Address:      parseUint16(ff[2]),
			Datatype:     ff[4],
			RegisterType: ff[5],
		}
		registers = append(registers, reg)
	}

	return registers, nil
}

func readDSL(r io.Reader) []string {
	scanner := bufio.NewScanner(r)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func parseInt64(s string) int64 {
	i, err := strconv.ParseInt(s, 16, 64)
	if err != nil {
		return i
	}
	return i
}

func parseUint16(s string) uint16 {
	i, err := strconv.ParseUint(s, 16, 16)
	if err != nil {
		return 0
	}
	return uint16(i)
}
