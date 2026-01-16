package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	bookFile       = "quijote.html"
	stateFile      = ".quijote_state.json"
	pageLineCount  = 25
	pageLineWidth  = 80
	pageSeparator  = "\n---\n"
	paragraphBreak = "\n\n"
)

type Chapter struct {
	Title     string
	Anchor    string
	Text      string
	StartPage int
}

type Book struct {
	Chapters []Chapter
	Pages    []string
}

type State struct {
	Chapter int `json:"chapter,omitempty"`
	Page    int `json:"page"`
}

type mode int

const (
	modeList mode = iota
	modeReader
)

type chapterItem struct {
	title string
	index int
}

func (c chapterItem) Title() string       { return c.title }
func (c chapterItem) Description() string { return "" }
func (c chapterItem) FilterValue() string { return c.title }

type errMsg struct{ err error }

func main() {
	if len(os.Args) < 2 {
		runUI()
		return
	}

	cmd := os.Args[1]
	switch cmd {
	case "list":
		runList()
	case "read":
		runRead(os.Args[2:])
	case "status":
		runStatus()
	case "goto":
		runGoto(os.Args[2:])
	case "reset":
		runReset()
	case "ui":
		runUI()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Comando desconocido: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	msg := `Uso:
  quijote             (interfaz interactiva)
  quijote ui
  quijote list
  quijote read [-n paginas]
  quijote status
  quijote goto <numero-capitulo>
  quijote reset
`
	fmt.Print(msg)
}

func runUI() {
	chapters, err := loadChapters(bookFile)
	if err != nil {
		exitErr(err)
	}
	pages, chapters := buildBookPagesForSize(chapters, pageLineWidth, pageLineCount)
	book := Book{Chapters: chapters, Pages: pages}

	state, _ := loadState(stateFile)
	if state.Page < 0 {
		state.Page = 0
	}
	if state.Page >= len(book.Pages) {
		state.Page = len(book.Pages) - 1
		if state.Page < 0 {
			state.Page = 0
		}
	}

	items := make([]list.Item, 0, len(book.Chapters))
	for i, ch := range book.Chapters {
		items = append(items, chapterItem{title: fmt.Sprintf("%3d. %s", i+1, ch.Title), index: i})
	}

	delegate := list.NewDefaultDelegate()
	chapterList := list.New(items, delegate, 0, 0)
	chapterList.Title = "Capitulos"
	chapterList.SetShowHelp(true)
	chapterList.SetFilteringEnabled(true)
	chapterList.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	chapterList.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	chapterList.Styles.FilterCursor = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))

	m := model{
		book:  book,
		state: state,
		mode:  modeReader,
		list:  chapterList,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		exitErr(err)
	}
}

func runList() {
	book, err := loadBook(bookFile)
	if err != nil {
		exitErr(err)
	}

	for i, ch := range book.Chapters {
		fmt.Printf("%3d. %s\n", i+1, ch.Title)
	}
}

func runStatus() {
	book, err := loadBook(bookFile)
	if err != nil {
		exitErr(err)
	}

	state, _ := loadState(stateFile)
	if len(book.Pages) == 0 {
		fmt.Println("No se encontraron paginas.")
		return
	}
	if state.Page >= len(book.Pages) {
		fmt.Println("Fin del libro.")
		return
	}

	chIdx := chapterIndexForPage(book.Chapters, state.Page)
	ch := book.Chapters[chIdx]
	fmt.Printf("Capitulo %d/%d: %s\n", chIdx+1, len(book.Chapters), ch.Title)
	fmt.Printf("Pagina %d/%d\n", state.Page+1, len(book.Pages))
}

func runRead(args []string) {
	fs := flag.NewFlagSet("read", flag.ExitOnError)
	pages := fs.Int("n", 1, "numero de paginas a leer")
	_ = fs.Parse(args)
	if *pages < 1 {
		exitErr(errors.New("paginas debe ser al menos 1"))
	}

	book, err := loadBook(bookFile)
	if err != nil {
		exitErr(err)
	}

	state, _ := loadState(stateFile)
	if state.Page >= len(book.Pages) {
		fmt.Println("Fin del libro. Usa 'quijote reset' para empezar de nuevo.")
		return
	}

	printed := 0
	for printed < *pages && state.Page < len(book.Pages) {
		if printed > 0 {
			fmt.Print(pageSeparator)
		}
		chIdx := chapterIndexForPage(book.Chapters, state.Page)
		fmt.Printf("Capitulo %d/%d: %s\n", chIdx+1, len(book.Chapters), book.Chapters[chIdx].Title)
		fmt.Printf("Pagina %d/%d\n\n", state.Page+1, len(book.Pages))
		fmt.Print(book.Pages[state.Page])
		state.Page++
		printed++
	}

	if err := saveState(stateFile, state); err != nil {
		exitErr(err)
	}
}

func runGoto(args []string) {
	if len(args) != 1 {
		exitErr(errors.New("uso: quijote goto <numero-capitulo>"))
	}

	book, err := loadBook(bookFile)
	if err != nil {
		exitErr(err)
	}

	idx, err := parseIndex(args[0])
	if err != nil {
		exitErr(err)
	}
	if idx < 1 || idx > len(book.Chapters) {
		exitErr(fmt.Errorf("numero de capitulo fuera de rango (1-%d)", len(book.Chapters)))
	}

	state := State{Chapter: idx - 1, Page: book.Chapters[idx-1].StartPage}
	if err := saveState(stateFile, state); err != nil {
		exitErr(err)
	}

	fmt.Printf("Capitulo establecido en %d: %s\n", idx, book.Chapters[idx-1].Title)
}

func runReset() {
	state := State{Chapter: 0, Page: 0}
	if err := saveState(stateFile, state); err != nil {
		exitErr(err)
	}
	fmt.Println("Progreso reiniciado al inicio.")
}

type model struct {
	book      Book
	state     State
	mode      mode
	list      list.Model
	width     int
	height    int
	pageWidth int
	pageLines int
	err       error
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case modeList:
			switch msg.String() {
			case "enter":
				if item, ok := m.list.SelectedItem().(chapterItem); ok {
					m.state.Page = m.book.Chapters[item.index].StartPage
					m.state.Chapter = item.index
					m.mode = modeReader
					return m, saveStateCmd(m.state)
				}
			case "q", "esc", "ctrl+c":
				return m, tea.Quit
			}
		case modeReader:
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "l":
				m.mode = modeList
				return m, nil
			case "enter", " ", "right", "down", "pgdown":
				if m.state.Page < len(m.book.Pages)-1 {
					m.state.Page++
					return m, saveStateCmd(m.state)
				}
			case "left", "up", "pgup", "b":
				if m.state.Page > 0 {
					m.state.Page--
					return m, saveStateCmd(m.state)
				}
			case "home":
				m.state.Page = 0
				return m, saveStateCmd(m.state)
			case "end":
				if len(m.book.Pages) > 0 {
					m.state.Page = len(m.book.Pages) - 1
					return m, saveStateCmd(m.state)
				}
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height)
		pageWidth, pageLines := computePageLayout(msg.Width, msg.Height)
		if pageWidth != m.pageWidth || pageLines != m.pageLines {
			oldTotal := len(m.book.Pages)
			oldPage := m.state.Page
			m.pageWidth = pageWidth
			m.pageLines = pageLines
			m.book.Pages, m.book.Chapters = buildBookPagesForSize(m.book.Chapters, m.pageWidth, m.pageLines)
			if oldTotal > 0 && len(m.book.Pages) > 0 {
				m.state.Page = remapPage(oldPage, oldTotal, len(m.book.Pages))
			} else if len(m.book.Pages) > 0 && m.state.Page >= len(m.book.Pages) {
				m.state.Page = len(m.book.Pages) - 1
			}
			return m, saveStateCmd(m.state)
		}
	case errMsg:
		m.err = msg.err
	}

	if m.mode == modeList {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	switch m.mode {
	case modeList:
		return m.list.View()
	case modeReader:
		return m.readerView()
	default:
		return ""
	}
}

func (m model) readerView() string {
	if len(m.book.Pages) == 0 {
		return "No se encontraron paginas."
	}
	page := m.book.Pages[m.state.Page]
	chIdx := chapterIndexForPage(m.book.Chapters, m.state.Page)
	chTitle := m.book.Chapters[chIdx].Title

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	footerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	header := titleStyle.Render("Don Quijote")
	chapterLine := metaStyle.Render(chTitle)
	status := metaStyle.Render(fmt.Sprintf("Pagina %d/%d", m.state.Page+1, len(m.book.Pages)))

	contentWidth := m.pageWidth
	if contentWidth == 0 {
		contentWidth = pageLineWidth
	}

	content := lipgloss.NewStyle().Width(contentWidth).Render(page)
	footer := footerStyle.Render("Enter/Espacio: siguiente  b/pgup: anterior  l: capitulos  q: salir")

	return strings.Join([]string{header, chapterLine, status, "", content, "", footer}, "\n")
}

func saveStateCmd(state State) tea.Cmd {
	return func() tea.Msg {
		if err := saveState(stateFile, state); err != nil {
			return errMsg{err: err}
		}
		return nil
	}
}

func loadBook(path string) (Book, error) {
	chapters, err := loadChapters(path)
	if err != nil {
		return Book{}, err
	}

	pages, chapters := buildBookPages(chapters)
	return Book{Chapters: chapters, Pages: pages}, nil
}

func loadChapters(path string) ([]Chapter, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`(?s)<h3><a name="([^"]+)"></a>(.*?)</h3>`)
	matches := re.FindAllSubmatchIndex(data, -1)
	if len(matches) == 0 {
		return nil, errors.New("no se encontraron capitulos en el HTML")
	}

	chapters := make([]Chapter, 0, len(matches))
	for i, m := range matches {
		anchor := string(data[m[2]:m[3]])
		title := cleanInlineText(string(data[m[4]:m[5]]))

		start := m[1]
		end := len(data)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}

		chunk := string(data[start:end])
		text := cleanHTMLToText(chunk)
		chapters = append(chapters, Chapter{Title: title, Anchor: anchor, Text: text})
	}

	return chapters, nil
}

func buildBookPages(chapters []Chapter) ([]string, []Chapter) {
	return buildBookPagesForSize(chapters, pageLineWidth, pageLineCount)
}

func buildBookPagesForSize(chapters []Chapter, width, lines int) ([]string, []Chapter) {
	var pages []string
	if width < 20 {
		width = 20
	}
	if lines < 5 {
		lines = 5
	}
	for i := range chapters {
		chapters[i].StartPage = len(pages)
		header := fmt.Sprintf("%s\n\n", chapters[i].Title)
		text := strings.TrimSpace(header + chapters[i].Text)
		chapterPages := paginate(text, lines, width)
		pages = append(pages, chapterPages...)
	}
	return pages, chapters
}

func chapterIndexForPage(chapters []Chapter, page int) int {
	if len(chapters) == 0 {
		return 0
	}
	idx := 0
	for i := range chapters {
		if chapters[i].StartPage > page {
			break
		}
		idx = i
	}
	return idx
}

func cleanInlineText(input string) string {
	text := stripTags(input)
	text = html.UnescapeString(text)
	return strings.TrimSpace(text)
}

func cleanHTMLToText(input string) string {
	normalized := strings.ReplaceAll(input, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	// Preserve paragraph and line breaks before stripping tags.
	normalized = replaceAllTag(normalized, "br", "\n")
	normalized = replaceAllTag(normalized, "/p", paragraphBreak)
	normalized = replaceAllTag(normalized, "p", "")
	normalized = replaceAllTag(normalized, "hr", "\n")

	text := stripTags(normalized)
	text = html.UnescapeString(text)
	text = normalizeWhitespace(text)
	return text
}

func replaceAllTag(input, tag, replacement string) string {
	re := regexp.MustCompile(`(?i)<\s*` + regexp.QuoteMeta(tag) + `\b[^>]*>`)
	return re.ReplaceAllString(input, replacement)
}

func stripTags(input string) string {
	var b strings.Builder
	b.Grow(len(input))
	inTag := false
	for _, r := range input {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func normalizeWhitespace(input string) string {
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(compactSpaces(line))
	}
	output := strings.Join(lines, "\n")

	// Collapse excessive blank lines to a single empty line.
	re := regexp.MustCompile(`\n{3,}`)
	output = re.ReplaceAllString(output, paragraphBreak)
	return strings.TrimSpace(output)
}

func compactSpaces(input string) string {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}

func paginate(text string, linesPerPage, lineWidth int) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	wrapped := wrapText(text, lineWidth)
	lines := strings.Split(wrapped, "\n")
	pages := []string{}
	for i := 0; i < len(lines); i += linesPerPage {
		end := i + linesPerPage
		if end > len(lines) {
			end = len(lines)
		}
		page := strings.Join(lines[i:end], "\n")
		pages = append(pages, strings.TrimSpace(page))
	}
	return pages
}

func wrapText(text string, width int) string {
	parts := strings.Split(text, paragraphBreak)
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, wrapParagraph(p, width))
	}
	return strings.Join(out, paragraphBreak)
}

func wrapParagraph(text string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	var b strings.Builder
	lineLen := 0
	for _, w := range words {
		if lineLen == 0 {
			b.WriteString(w)
			lineLen = len(w)
			continue
		}
		if lineLen+1+len(w) > width {
			b.WriteByte('\n')
			b.WriteString(w)
			lineLen = len(w)
			continue
		}
		b.WriteByte(' ')
		b.WriteString(w)
		lineLen += 1 + len(w)
	}

	return b.String()
}

func loadState(path string) (State, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{Chapter: 0, Page: 0}, nil
		}
		return State{}, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return State{}, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, err
	}
	return state, nil
}

func saveState(path string, state State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func parseIndex(input string) (int, error) {
	var idx int
	_, err := fmt.Sscanf(input, "%d", &idx)
	if err != nil {
		return 0, fmt.Errorf("invalid chapter number: %s", input)
	}
	return idx, nil
}

func computePageLayout(width, height int) (int, int) {
	pageWidth := pageLineWidth
	if width > 0 {
		pageWidth = width - 4
		if pageWidth < 40 {
			pageWidth = 40
		}
	}
	pageLines := pageLineCount
	if height > 0 {
		pageLines = height - 8
		if pageLines < 10 {
			pageLines = 10
		}
	}
	return pageWidth, pageLines
}

func remapPage(oldPage, oldTotal, newTotal int) int {
	if oldTotal <= 0 || newTotal <= 0 {
		return 0
	}
	progress := float64(oldPage) / float64(oldTotal)
	newPage := int(progress * float64(newTotal))
	if newPage < 0 {
		newPage = 0
	}
	if newPage >= newTotal {
		newPage = newTotal - 1
	}
	return newPage
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
