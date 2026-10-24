package cpc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

const publicEndpoint = "https://api.ipify.org"

type menuRow struct {
	id, title, description, keywords, value, preview string
	loading, hidden                                  bool
	err                                              error
	children                                         []item
	script                                           *scriptValues
}
type previewMsg struct {
	id, value string
	children  []item
	err       error
	hidden    bool
}
type scriptPage struct {
	rows             []menuRow
	input            textinput.Model
	selected, offset int
}
type menuModel struct {
	ctx                             context.Context
	lang                            language
	cwd, page                       string
	rows, mainRows                  []menuRow
	input                           textinput.Model
	selected, offset, width, height int
	result                          string
	done, cancelled                 bool
	notice                          string
	endpoint                        string
	scriptBack                      *scriptPage
	scriptFormat, scriptName        string
}

func baseMenu(ctx context.Context, l language) menuModel {
	input := textinput.New()
	input.Prompt = l.text("搜索: ", "Search: ")
	input.Focus()
	input.CharLimit = 512
	return menuModel{ctx: ctx, lang: l, input: input, width: 80, height: 24, endpoint: publicEndpoint}
}
func newMainMenu(ctx context.Context, cwd string, l language) menuModel {
	m := baseMenu(ctx, l)
	m.cwd = cwd
	definitions := []struct{ id, cn, en, cnDesc, enDesc string }{
		{"ip", "本地 IP", "Local IP", "复制本机网卡地址", "Copy a local network address"},
		{"public-ip", "外网 IP", "Public IP", "复制公网地址；失败后 Enter 重试", "Copy your public address; Enter to retry on failure"},
		{"pwd", "工作目录", "Working directory", "复制当前绝对路径", "Copy the current absolute path"},
		{"branch", "Git 分支", "Git branch", "复制当前分支名", "Copy the current branch name"},
		{"commit", "提交哈希", "Commit hash", "复制完整 Git HEAD 哈希", "Copy the full Git HEAD commit hash"},
		{"script", "项目脚本", "Project scripts", "选择任务及要复制的内容", "Select a task and what to copy"},
	}
	for _, d := range definitions {
		r := menuRow{id: d.id, title: l.text(d.cn, d.en), description: l.text(d.cnDesc, d.enDesc), keywords: d.cn + " " + d.en + " " + d.cnDesc + " " + d.enDesc, loading: true}
		if d.id == "pwd" {
			r.loading = false
			r.value = cwd
			r.preview = cwd
		}
		if d.id == "script" {
			r.hidden = true
		}
		m.rows = append(m.rows, r)
	}
	return m
}
func childRows(kind string, entries []item) []menuRow {
	rows := make([]menuRow, 0, len(entries))
	for _, e := range entries {
		r := menuRow{id: e.Key, title: e.Title, value: e.copyValue(), preview: e.Description, description: e.Description, keywords: e.Key + " " + e.Title + " " + e.Description, script: e.Script}
		if kind == "ip" {
			r.preview = e.Key + " · " + e.Description
		}
		rows = append(rows, r)
	}
	return rows
}
func newChildMenu(ctx context.Context, l language, kind string, entries []item) menuModel {
	m := baseMenu(ctx, l)
	m.page = kind
	m.rows = childRows(kind, entries)
	return m
}
func (m *menuModel) openScriptCopy(task menuRow) {
	m.scriptBack = &scriptPage{rows: m.rows, input: m.input, selected: m.selected, offset: m.offset}
	m.scriptName = task.title
	s := task.script
	m.rows = []menuRow{
		{id: "name", title: m.lang.text("脚本名字", "Script name"), value: s.Name, preview: s.Name, description: m.lang.text("复制原始任务名", "Copy the original task name"), keywords: "name script 名字 名称"},
		{id: "command", title: m.lang.text("执行命令", "Run command"), value: s.Command, preview: s.Command, description: m.lang.text("复制运行该任务的完整命令", "Copy the command that runs this task"), keywords: "command run 执行 命令"},
		{id: "content", title: m.lang.text("脚本内容", "Script content"), value: s.Content, preview: s.Content, description: m.lang.text("复制脚本定义的原始内容", "Copy the original script content"), keywords: "content body 内容"},
	}
	m.page, m.selected, m.offset, m.notice = "script-copy", 0, 0, ""
	// Use a fresh input so editing this page doesn't alter the parent's query buffer.
	m.input = baseMenu(m.ctx, m.lang).input
	m.input.Width = max(1, m.width-ansi.StringWidth(m.input.Prompt)-2)
}

func (m menuModel) load(id string) tea.Cmd {
	ctx, cwd, endpoint := m.ctx, m.cwd, m.endpoint
	return func() tea.Msg {
		msg := previewMsg{id: id}
		switch id {
		case "branch", "commit":
			msg.value, msg.err = gitValue(ctx, cwd, id)
		case "public-ip":
			msg.value, msg.err = publicIP(ctx, endpoint)
		case "ip":
			msg.children, msg.err = localIPs()
			if msg.err == nil && len(msg.children) == 0 {
				msg.err = problem("没有可用的本地 IP", "No usable local IP")
			}
		case "script":
			projects, err := findProjects(cwd)
			if err != nil {
				msg.hidden = errors.Is(err, errNoProject)
				msg.err = err
				return msg
			}
			names := make([]string, 0, len(projects))
			for _, p := range projects {
				names = append(names, map[string]string{"node": "Node", "go": "Go", "rust": "Rust"}[p.kind])
			}
			msg.value = strings.Join(names, " / ")
			msg.children, msg.err = projectScripts(projects)
		}
		return msg
	}
}
func (m menuModel) Init() tea.Cmd {
	if m.page != "" {
		return textinput.Blink
	}
	return tea.Batch(textinput.Blink, m.load("ip"), m.load("public-ip"), m.load("branch"), m.load("commit"), m.load("script"))
}
func (m menuModel) visible() []int {
	indices := []int{}
	for i, r := range m.rows {
		if !r.hidden && matches(m.input.Value(), item{Key: r.id, Title: r.title, Description: r.keywords + " " + r.value + " " + r.preview}) {
			indices = append(indices, i)
		}
	}
	return indices
}
func (m menuModel) selectedID() string {
	v := m.visible()
	if m.selected >= 0 && m.selected < len(v) {
		return m.rows[v[m.selected]].id
	}
	return ""
}
func (m *menuModel) restoreSelection(id string) {
	v := m.visible()
	for i, index := range v {
		if m.rows[index].id == id {
			m.selected = i
			return
		}
	}
	m.selected = min(m.selected, max(0, len(v)-1))
}
func (m *menuModel) apply(msg previewMsg) {
	rows := m.rows
	if m.mainRows != nil {
		rows = m.mainRows
	}
	for i := range rows {
		r := &rows[i]
		if r.id != msg.id {
			continue
		}
		r.loading = false
		r.err = msg.err
		r.hidden = msg.hidden
		r.value = msg.value
		r.preview = msg.value
		r.children = msg.children
		if msg.id == "commit" && len(msg.value) > 12 {
			r.preview = msg.value[:12]
		}
		if msg.id == "ip" && len(msg.children) > 0 {
			r.preview = msg.children[0].Key
			if len(msg.children) == 1 {
				r.value = msg.children[0].Key
			} else {
				r.preview += fmt.Sprintf(m.lang.text("（%d 个地址）", " (%d addresses)"), len(msg.children))
			}
		}
		if msg.id == "script" {
			r.preview = fmt.Sprintf(m.lang.text("%d 个任务", "%d tasks"), len(msg.children))
			if msg.value != "" {
				r.preview = msg.value + " · " + r.preview
			}
			r.value = ""
		}
	}
}
func (m menuModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width = max(1, msg.Width)
		m.height = max(1, msg.Height)
		m.input.Width = max(1, m.width-ansi.StringWidth(m.input.Prompt)-2)
		return m, nil
	case previewMsg:
		id := m.selectedID()
		m.apply(msg)
		if m.page == "" && id == msg.id {
			m.notice = ""
		}
		m.restoreSelection(id)
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+d":
			m.cancelled = true
			return m, tea.Quit
		case "esc":
			if m.scriptBack != nil {
				back := m.scriptBack
				m.rows, m.input, m.selected, m.offset = back.rows, back.input, back.selected, back.offset
				m.input.Width = max(1, m.width-ansi.StringWidth(m.input.Prompt)-2)
				m.page, m.scriptBack, m.notice = "script", nil, ""
				return m, nil
			}
			if m.mainRows != nil {
				m.rows = m.mainRows
				m.mainRows = nil
				m.page = ""
				m.input.SetValue("")
				m.selected = 0
				m.offset = 0
				m.notice = ""
				return m, nil
			}
			m.cancelled = true
			return m, tea.Quit
		case "up", "ctrl+p":
			m.selected = max(0, m.selected-1)
			m.notice = ""
			return m, nil
		case "down", "ctrl+n":
			m.selected = min(max(0, len(m.visible())-1), m.selected+1)
			m.notice = ""
			return m, nil
		case "enter":
			visible := m.visible()
			if len(visible) == 0 {
				return m, nil
			}
			r := &m.rows[visible[m.selected]]
			if r.loading {
				m.notice = m.lang.text("正在加载，请稍候", "Loading, please wait")
				return m, nil
			}
			if r.err != nil {
				if r.id == "public-ip" && m.page == "" {
					r.loading = true
					r.err = nil
					m.notice = ""
					return m, m.load(r.id)
				}
				m.notice = errorText(m.lang, r.err)
				return m, nil
			}
			if m.page == "script" && r.script != nil {
				if m.scriptFormat != "" {
					m.result = r.script.value(m.scriptFormat)
					m.done = true
					return m, tea.Quit
				}
				m.openScriptCopy(*r)
				return m, nil
			}
			if len(r.children) > 0 && (r.id == "script" || len(r.children) > 1) {
				kind, entries := r.id, r.children
				m.mainRows = m.rows
				m.rows = childRows(kind, entries)
				m.page = kind
				m.selected = 0
				m.offset = 0
				m.input.SetValue("")
				m.notice = ""
				return m, nil
			}
			m.result = r.value
			m.done = true
			return m, tea.Quit
		}
	}
	previous := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(message)
	if m.input.Value() != previous {
		m.selected = 0
		m.offset = 0
		m.notice = ""
	}
	return m, cmd
}

func clip(s string, width int) string { return ansi.Truncate(display(s), max(0, width), "…") }
func (m menuModel) View() string {
	if m.done || m.cancelled {
		return ""
	}
	width := max(1, m.width-1) // Leave a spare column to avoid terminal auto-wrap.
	title := m.lang.text("复制上下文", "Copy context")
	if m.page == "script" {
		title = m.lang.text("选择脚本", "Select script")
	}
	if m.page == "script-copy" {
		title = m.lang.text("选择复制内容", "Choose what to copy") + " · " + m.scriptName
	}
	if m.page == "ip" {
		title = m.lang.text("选择本地 IP", "Select local IP")
	}
	visible := m.visible()
	lines := []string{clip(title, width), ansi.Truncate(m.input.View(), width, "…")}
	capacity := max(1, m.height-5)
	offset := m.offset
	if m.selected < offset {
		offset = m.selected
	}
	if m.selected >= offset+capacity {
		offset = m.selected - capacity + 1
	}
	if len(visible) == 0 {
		lines = append(lines, clip(m.lang.text("没有匹配结果", "No matches"), width))
	}
	for n := offset; n < min(len(visible), offset+capacity); n++ {
		r := m.rows[visible[n]]
		value := r.preview
		if r.loading {
			value = m.lang.text("加载中…", "Loading…")
		}
		if r.err != nil {
			value = m.lang.text("不可用", "Unavailable")
		}
		prefix := "  "
		if n == m.selected {
			prefix = "› "
		}
		// Reserve space for the actual value; command IDs are redundant in narrow terminals.
		name := r.title
		if width >= 70 && m.page == "" {
			name += " · " + r.id
		}
		nameWidth := min(ansi.StringWidth(name), max(8, width/3))
		line := prefix + clip(name, nameWidth) + "  " + clip(value, width-nameWidth-4)
		line = clip(line, width)
		if n == m.selected {
			line = "\x1b[36m" + line + "\x1b[0m"
		}
		lines = append(lines, line)
	}
	detail := m.notice
	if detail == "" && len(visible) > 0 {
		r := m.rows[visible[m.selected]]
		detail = r.description
		if r.err != nil {
			detail = errorText(m.lang, r.err)
		}
	}
	lines = append(lines, clip(detail, width))
	footer := m.lang.text("↑↓ / Ctrl+N/P 选择 · Enter 确认 · Esc 返回 · Ctrl+C 取消", "↑↓ / Ctrl+N/P move · Enter select · Esc back · Ctrl+C cancel")
	if width < 70 {
		footer = "↑↓ ^N/^P · Enter · Esc · ^C"
	}
	lines = append(lines, clip(footer, width))
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	return strings.Join(lines, "\n")
}

func runMenu(ctx context.Context, m menuModel) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stderr.Fd())) {
		return "", problem("交互选择需要终端；请使用 cpc --help", "Interactive selection requires a terminal; use cpc --help")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	m.ctx = ctx
	if w, h, err := term.GetSize(int(os.Stderr.Fd())); err == nil {
		m.width = max(1, w)
		m.height = max(1, h)
	}
	m.input.Width = max(1, m.width-ansi.StringWidth(m.input.Prompt)-2)
	model, err := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(os.Stdin), tea.WithOutput(os.Stderr), tea.WithAltScreen()).Run()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", err
	}
	result := model.(menuModel)
	if result.cancelled || !result.done {
		return "", context.Canceled
	}
	return result.result, nil
}
