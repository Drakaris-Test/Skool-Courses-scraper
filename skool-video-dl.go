package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io/fs"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// -----------------------------------------------------------------------------
// Constantes + Types
// -----------------------------------------------------------------------------
const (
	browserTimeout   = 1800 * time.Second
	defaultWaitTime  = 5
	defaultOutputDir = "downloads"
	defaultHeadless  = true

	skoolLoginURL = "https://www.skool.com/login"
)

type Config struct {
	SkoolURL  string
	Email     string
	Password  string
	OutputDir string
	Wait      int
	Headless  bool
	Debug     bool
}

type Course struct {
	Title string
	URL   string
}
type ModuleInfo struct {
	ID    string
	Title string
	URL   string
}
type CourseData struct {
	Title   string
	URL     string
	Modules []ModuleData
}
type ModuleData struct {
	Title       string
	URL         string
	Description string
	Videos      []VideoRecord
}
type VideoRecord struct {
	URL      string
	Filename string
}

type Mark struct {
	Type  string                 `json:"type"`
	Attrs map[string]interface{} `json:"attrs,omitempty"`
}
type TiptapNode struct {
	Type    string                 `json:"type"`
	Text    string                 `json:"text,omitempty"`
	Marks   []Mark                 `json:"marks,omitempty"`
	Attrs   map[string]interface{} `json:"attrs,omitempty"`
	Content []TiptapNode           `json:"content,omitempty"`
}

// -----------------------------------------------------------------------------
// MAIN
// -----------------------------------------------------------------------------
func main() {
	cfg := parseFlags()
	initLogging(cfg.Debug)
	printBanner()
	must(os.MkdirAll(cfg.OutputDir, fs.ModePerm))
	ctx, cancel := setupBrowser(cfg.Headless)
	defer cancel()

	if err := loginWithCreds(ctx, cfg.Email, cfg.Password); err != nil {
		log.Fatalf("❌ login failed: %v", err)
	}

	courses, err := scrapeCourses(ctx, cfg)
	if err != nil {
		log.Fatalf("❌ cannot list courses: %v", err)
	}
	fmt.Printf("🗂️  Found %d course(s)\n", len(courses))

	var allCourses []CourseData
	for i, c := range courses {
		fmt.Printf("\n[%d/%d] ➜ %s\n", i+1, len(courses), c.Title)
		courseDir := filepath.Join(cfg.OutputDir, c.Title)
		must(os.MkdirAll(courseDir, fs.ModePerm))

		mods, err := scrapeModulesForCourse(ctx, c.URL, cfg)
		if err != nil {
			fmt.Printf("  ⚠️  cannot list modules: %v\n", err)
			continue
		}
		fmt.Printf("  📚 Found %d module(s)\n", len(mods))

		var moduleDatas []ModuleData
		for j, m := range mods {
			fmt.Printf("  [%d/%d] ➜ %s\n", j+1, len(mods), m.Title)
			modData, err := handleModule(ctx, m, courseDir, cfg)
			if err != nil {
				fmt.Printf("    ⚠️  %v\n", err)
			}
			moduleDatas = append(moduleDatas, modData)
		}

		allCourses = append(allCourses, CourseData{
			Title:   c.Title,
			URL:     c.URL,
			Modules: moduleDatas,
		})
	}

	fmt.Println("\n✅ All done!")
	buildHTMLIndex(allCourses, cfg.OutputDir)
	fmt.Printf("📁 Created %s/index.html\n", cfg.OutputDir)
}

// -----------------------------------------------------------------------------
// parseFlags + logging + banner
// -----------------------------------------------------------------------------
func parseFlags() Config {
	var c Config
	flag.StringVar(&c.SkoolURL, "url", "", "Skool classroom URL (required)")
	flag.StringVar(&c.Email, "email", "", "Email for Skool login")
	flag.StringVar(&c.Password, "password", "", "Password for Skool login")
	flag.StringVar(&c.OutputDir, "output", defaultOutputDir, "Download directory")
	flag.IntVar(&c.Wait, "wait", defaultWaitTime, "Wait time (seconds) after nav")
	flag.BoolVar(&c.Headless, "headless", defaultHeadless, "Run Chrome headless")
	flag.BoolVar(&c.Debug, "debug", false, "Show debug logs")
	flag.Parse()

	if c.SkoolURL == "" {
		log.Fatal("missing -url")
	}
	if c.Email == "" || c.Password == "" {
		log.Fatal("missing -email/-password")
	}
	return c
}
func initLogging(debug bool) {
	if debug {
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	} else {
		log.SetFlags(0)
		log.SetOutput(os.Stdout)
	}
}
func printBanner() {
	fmt.Println(`
████████╗ ██████╗  ██████╗ ██╗     
╚══██╔══╝██╔═══██╗██╔═══██╗██║     
   ██║   ██║   ██║██║   ██║██║     
   ██║   ██║   ██║██║   ██║██║     
   ██║   ╚██████╔╝╚██████╔╝███████╗
   ╚═╝    ╚═════╝  ╚═════╝ ╚══════╝
  ███████╗███╗   ██╗██╗██╗   ██╗██╗     
  ██╔════╝████╗  ██║██║██║   ██║██║     
  █████╗  ██╔██╗ ██║██║██║   ██║██║     
  ██╔══╝  ██║╚██╗██║██║╚██╗ ██╔╝██║     
  ███████╗██║ ╚████║██║ ╚████╔╝ ███████╗
  ╚══════╝╚═╝  ╚═══╝╚═╝  ╚═══╝  ╚══════╝
              TOOL BY SNIV3L
`)
}
func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// -----------------------------------------------------------------------------
// Setup + login
// -----------------------------------------------------------------------------
func setupBrowser(headless bool) (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 "+
			"(KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36"),
	)
	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancelAlloc := chromedp.NewContext(allocCtx)
	ctx, cancelTimeout := context.WithTimeout(ctx, browserTimeout)

	return ctx, func() {
		cancelTimeout()
		cancelAlloc()
	}
}

func loginWithCreds(ctx context.Context, email, pass string) error {
	return chromedp.Run(ctx,
		chromedp.Navigate(skoolLoginURL),
		chromedp.WaitVisible(`input[type="email"]`),
		chromedp.SendKeys(`input[type="email"]`, email),
		chromedp.SendKeys(`input[type="password"]`, pass),
		chromedp.Click(`button[type="submit"]`),
		chromedp.Sleep(4*time.Second),
	)
}

// -----------------------------------------------------------------------------
// scrapeCourses => lit __NEXT_DATA__ => pageProps.allCourses
// -----------------------------------------------------------------------------
func scrapeCourses(ctx context.Context, cfg Config) ([]Course, error) {
	if err := chromedp.Run(ctx,
		chromedp.Navigate(cfg.SkoolURL),
		chromedp.Sleep(time.Duration(cfg.Wait)*time.Second),
	); err != nil {
		return nil, err
	}
	var raw string
	if err := chromedp.Run(ctx,
		chromedp.WaitReady(`#__NEXT_DATA__`),
		chromedp.EvaluateAsDevTools(
			`document.getElementById("__NEXT_DATA__").textContent`, &raw,
		),
	); err != nil {
		return nil, err
	}

	var data struct {
		Props struct {
			PageProps struct {
				AllCourses []struct {
					Name     string `json:"name"`
					Metadata struct {
						Title string `json:"title"`
					} `json:"metadata"`
				} `json:"allCourses"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if e := json.Unmarshal([]byte(raw), &data); e != nil {
		return nil, e
	}
	var out []Course
	for _, c := range data.Props.PageProps.AllCourses {
		title := clean(c.Metadata.Title)
		url := strings.TrimRight(cfg.SkoolURL, "/") + "/" + c.Name
		out = append(out, Course{Title: title, URL: url})
	}
	return out, nil
}

// -----------------------------------------------------------------------------
// scrapeModulesForCourse => children => ID + title
// -----------------------------------------------------------------------------
func scrapeModulesForCourse(ctx context.Context, courseURL string, cfg Config) ([]ModuleInfo, error) {
	if err := chromedp.Run(ctx,
		chromedp.Navigate(courseURL),
		chromedp.Sleep(time.Duration(cfg.Wait)*time.Second),
	); err != nil {
		return nil, err
	}
	var raw string
	if err := chromedp.Run(ctx,
		chromedp.WaitReady(`#__NEXT_DATA__`),
		chromedp.EvaluateAsDevTools(`document.getElementById("__NEXT_DATA__").textContent`, &raw),
	); err != nil {
		return nil, err
	}
	var data struct {
		Props struct {
			PageProps struct {
				Course struct {
					Children []struct {
						Course struct {
							Id       string `json:"id"`
							Metadata struct {
								Title string `json:"title"`
							} `json:"metadata"`
						} `json:"course"`
					} `json:"children"`
				} `json:"course"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if e := json.Unmarshal([]byte(raw), &data); e != nil {
		return nil, e
	}
	var ms []ModuleInfo
	for _, c := range data.Props.PageProps.Course.Children {
		id := c.Course.Id
		t := c.Course.Metadata.Title
		if t == "" {
			t = "Untitled"
		}
		u := courseURL + "?md=" + id
		ms = append(ms, ModuleInfo{ID: id, Title: clean(t), URL: u})
	}
	return ms, nil
}

// -----------------------------------------------------------------------------
// handleModule => parse Tiptap => bullet => build HTML
// -----------------------------------------------------------------------------
func handleModule(ctx context.Context, m ModuleInfo, courseDir string, cfg Config) (ModuleData, error) {
	modDir := filepath.Join(courseDir, m.Title)
	modFile := filepath.Join(modDir, "module.html")

	// Skip module if HTML already exists and is non-empty
	if fileExistsAndNonZero(modFile) {
		fmt.Println("    already downloaded, skipping")
		return ModuleData{Title: m.Title, URL: m.URL}, nil
	}

	must(os.MkdirAll(modDir, fs.ModePerm))

	if err := chromedp.Run(ctx,
		chromedp.Navigate(m.URL),
		chromedp.Sleep(time.Duration(cfg.Wait)*time.Second),
	); err != nil {
		return ModuleData{}, err
	}
	var raw string
	if err := chromedp.Run(ctx,
		chromedp.WaitReady(`#__NEXT_DATA__`),
		chromedp.EvaluateAsDevTools(`document.getElementById("__NEXT_DATA__").textContent`, &raw),
	); err != nil {
		log.Printf("Cannot read __NEXT_DATA__ for module %s: %v\n", m.Title, err)
	}

	var data struct {
		Props struct {
			PageProps struct {
				Course struct {
					Children []map[string]interface{} `json:"children"`
				} `json:"course"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	_ = json.Unmarshal([]byte(raw), &data)

	var desc string
	var videoLinks []string
	var allLinks []string

	for _, ch := range data.Props.PageProps.Course.Children {
		course, _ := ch["course"].(map[string]interface{})
		if course == nil {
			continue
		}
		id, _ := course["id"].(string)
		if id != m.ID {
			continue
		}
		// Description (desc)
		metadata, _ := course["metadata"].(map[string]interface{})
		if metadata != nil {
			if d, ok := metadata["desc"].(string); ok {
				desc = d
			}
		}
		// Recherche récursive de tous les videoLink dans la structure complète du module
		videoLinks = extractAllVideoLinksFromAny(course)
		break
	}

	// Ajoute aussi les liens Loom/Vimeo dans le contenu (comme avant)
	links := extractLoomVimeoLinks(desc)
	for _, l := range videoLinks {
		allLinks = append(allLinks, rewriteVimeoToPlayer(l))
	}
	allLinks = append(allLinks, links...)
	allLinks = uniqueStrings(allLinks)

	descBullet := forceConvertTiptapBullet(desc)
	if strings.Contains(descBullet, "[{") || strings.Contains(descBullet, "\"type\":") {
		descBullet = "<p>" + html.EscapeString(strings.TrimSpace(desc)) + "</p>"
	}

	var recs []VideoRecord
	for i, link := range allLinks {
		tried := false
		for _, testURL := range allVimeoUrls(link) {
			fmt.Printf("    downloading => %s\n", testURL)
			fn, err := downloadVideo(testURL, modDir, i+1)
			if err == nil {
				recs = append(recs, VideoRecord{URL: testURL, Filename: fn})
				tried = true
				break
			} else {
				fmt.Printf("      ⚠️  fail dl: %v\n", err)
			}
		}
		if !tried {
			fmt.Printf("      ⚠️  all download attempts failed for: %s\n", link)
		}
	}

	if err := buildModuleHTML(modFile, m.Title, descBullet, recs); err != nil {
		log.Printf("Cannot write module.html for %s: %v\n", m.Title, err)
	}
	return ModuleData{
		Title:       m.Title,
		URL:         m.URL,
		Description: descBullet,
		Videos:      recs,
	}, nil
}

// -----------------------------------------------------------------------------
// Recherche récursive de tous les videoLink dans la structure
// -----------------------------------------------------------------------------
func extractAllVideoLinksFromAny(val interface{}) []string {
	var links []string
	switch v := val.(type) {
	case map[string]interface{}:
		for k, child := range v {
			if k == "videoLink" {
				if link, ok := child.(string); ok && link != "" {
					links = append(links, link)
				}
			}
			links = append(links, extractAllVideoLinksFromAny(child)...)
		}
	case []interface{}:
		for _, item := range v {
			links = append(links, extractAllVideoLinksFromAny(item)...)
		}
	}
	return links
}

// -----------------------------------------------------------------------------
// Essayer toutes les variantes d'URL Vimeo
// -----------------------------------------------------------------------------
func allVimeoUrls(link string) []string {
	id := ""
	hash := ""

	if u, err := url.Parse(link); err == nil {
		segs := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(segs) > 0 {
			if segs[0] == "video" {
				if len(segs) > 1 {
					id = segs[1]
				}
				if len(segs) > 2 {
					hash = segs[2]
				}
			} else {
				id = segs[0]
				if len(segs) > 1 {
					hash = segs[1]
				}
			}
		}
		if h := u.Query().Get("h"); h != "" {
			hash = h
		}
	} else {
		re := regexp.MustCompile(`vimeo\.com/(?:video/)?(\d+)(?:/([a-zA-Z0-9]+))?`)
		m := re.FindStringSubmatch(link)
		if len(m) > 1 {
			id = m[1]
		}
		if len(m) > 2 {
			hash = m[2]
		}
	}
	var urls []string
	if id != "" {
		if hash != "" {
			urls = append(urls, fmt.Sprintf("https://player.vimeo.com/video/%s?h=%s", id, hash))
		}
		urls = append(urls, fmt.Sprintf("https://player.vimeo.com/video/%s", id))
		if hash != "" {
			urls = append(urls, fmt.Sprintf("https://vimeo.com/%s/%s", id, hash))
		}
		urls = append(urls, fmt.Sprintf("https://vimeo.com/%s", id))
	}
	if !contains(urls, link) {
		urls = append(urls, link)
	}
	return urls
}
func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// Tiptap -> HTML natif lisible (p, h1, ul, li, a, strong, etc.)
// -----------------------------------------------------------------------------
func forceConvertTiptapBullet(desc string) string {
	if desc == "" {
		return ""
	}
	desc = strings.ReplaceAll(desc, "[v2]", "")
	idx := strings.Index(desc, "[{")
	if idx < 0 {
		return "<p>" + html.EscapeString(strings.TrimSpace(desc)) + "</p>"
	}
	j := desc[idx:]
	u := html.UnescapeString(j)

	var nodes []TiptapNode
	if err := json.Unmarshal([]byte(u), &nodes); err == nil {
		return strings.TrimSpace(renderTiptapNodesHTML(nodes))
	}
	var root struct {
		Content []TiptapNode `json:"content"`
	}
	if err := json.Unmarshal([]byte(u), &root); err == nil && len(root.Content) > 0 {
		return strings.TrimSpace(renderTiptapNodesHTML(root.Content))
	}
	return "<p>" + html.EscapeString(strings.TrimSpace(desc)) + "</p>"
}
func renderTiptapNodesHTML(nodes []TiptapNode) string {
	var sb strings.Builder
	for _, node := range nodes {
		sb.WriteString(renderTiptapNodeHTML(node))
	}
	return sb.String()
}
func renderTiptapNodeHTML(node TiptapNode) string {
	switch node.Type {
	case "heading":
		level := 1
		if l, ok := node.Attrs["level"].(float64); ok {
			level = int(l)
		}
		headingTag := fmt.Sprintf("h%d", level)
		text := strings.TrimSpace(renderTiptapNodesHTML(node.Content))
		return fmt.Sprintf("<%s>%s</%s>\n", headingTag, text, headingTag)
	case "paragraph":
		txt := strings.TrimSpace(renderTiptapNodesHTML(node.Content))
		if txt == "" {
			return ""
		}
		return "<p>" + txt + "</p>\n"
	case "bulletList":
		return "<ul>\n" + renderListItems(node.Content) + "</ul>\n"
	case "orderedList":
		return "<ol>\n" + renderListItems(node.Content) + "</ol>\n"
	case "listItem":
		return "<li>" + strings.TrimSpace(renderTiptapNodesHTML(node.Content)) + "</li>\n"
	case "text":
		txt := html.EscapeString(node.Text)
		for _, mark := range node.Marks {
			switch mark.Type {
			case "bold":
				txt = "<strong>" + txt + "</strong>"
			case "italic":
				txt = "<em>" + txt + "</em>"
			case "link":
				href, _ := mark.Attrs["href"].(string)
				txt = "<a href=\"" + html.EscapeString(href) + "\" target=\"_blank\">" + txt + "</a>"
			}
		}
		return txt
	case "hardBreak":
		return "<br>\n"
	case "blockquote":
		content := strings.TrimSpace(renderTiptapNodesHTML(node.Content))
		return "<blockquote>" + content + "</blockquote>\n"
	default:
		if len(node.Content) > 0 {
			return renderTiptapNodesHTML(node.Content)
		}
	}
	return ""
}
func renderListItems(nodes []TiptapNode) string {
	var sb strings.Builder
	for _, n := range nodes {
		sb.WriteString(renderTiptapNodeHTML(n))
	}
	return sb.String()
}

// -----------------------------------------------------------------------------
// extractLoomVimeoLinks => parse la version Node (HTML) pour .Marks => link
// -----------------------------------------------------------------------------
func extractLoomVimeoLinks(desc string) []string {
	if !strings.Contains(desc, "[{") {
		return nil
	}
	idx := strings.Index(desc, "[{")
	if idx < 0 {
		return nil
	}
	j := desc[idx:]
	u := html.UnescapeString(j)

	var nds []TiptapNode
	if err := json.Unmarshal([]byte(u), &nds); err == nil {
		return filterLinksFromTiptap(nds)
	}
	var root struct {
		Content []TiptapNode `json:"content"`
	}
	if err := json.Unmarshal([]byte(u), &root); err == nil && len(root.Content) > 0 {
		return filterLinksFromTiptap(root.Content)
	}
	return nil
}
func filterLinksFromTiptap(nodes []TiptapNode) []string {
	var out []string
	traverseNodesForLinks(nodes, &out)
	return filterLoomVimeo(uniqueStrings(out))
}
func traverseNodesForLinks(nodes []TiptapNode, out *[]string) {
	for _, n := range nodes {
		for _, mk := range n.Marks {
			if mk.Type == "link" {
				if href, ok := mk.Attrs["href"].(string); ok {
					*out = append(*out, href)
				}
			}
		}
		if len(n.Content) > 0 {
			traverseNodesForLinks(n.Content, out)
		}
	}
}
func filterLoomVimeo(links []string) []string {
	var out []string
	for _, s := range links {
		ll := strings.ToLower(s)
		if strings.Contains(ll, "loom.com") || strings.Contains(ll, "vimeo.com") {
			out = append(out, s)
		}
	}
	return out
}

// -----------------------------------------------------------------------------
// rewriteVimeoToPlayer => vimeo.com/\d+ => player
// -----------------------------------------------------------------------------
var reVimeoNum = regexp.MustCompile(`(?i)vimeo\.com/(?:video/)?(\d+)`)

func rewriteVimeoToPlayer(link string) string {
	if strings.Contains(link, "player.vimeo.com") {
		return link
	}

	u, err := url.Parse(link)
	if err != nil {
		return link
	}

	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	id := ""
	hash := u.Query().Get("h")

	if len(segs) > 0 {
		if segs[0] == "video" {
			if len(segs) > 1 {
				id = segs[1]
			}
			if len(segs) > 2 {
				hash = segs[2]
			}
		} else {
			id = segs[0]
			if len(segs) > 1 {
				hash = segs[1]
			}
		}
	}

	if id == "" {
		m := reVimeoNum.FindStringSubmatch(link)
		if len(m) > 1 {
			id = m[1]
		}
	}

	if id == "" {
		return link
	}

	if hash != "" {
		return fmt.Sprintf("https://player.vimeo.com/video/%s?h=%s", id, hash)
	}
	return fmt.Sprintf("https://player.vimeo.com/video/%s", id)
}

// -----------------------------------------------------------------------------
// buildModuleHTML => desc dans <div class="content">, liens natifs HTML
// -----------------------------------------------------------------------------
func buildModuleHTML(path string, title, desc string, videos []VideoRecord) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>%s</title>
  <style>
    body { font-family: Arial, sans-serif; line-height: 1.5; max-width: 800px; margin: 2rem auto; padding: 1rem; background-color: #fff; color: #333; }
    h1, h2, h3, h4, h5, h6 { margin-top: 1.2em; margin-bottom: 0.6em; color: #222; }
    .content { font-family: inherit; margin-bottom: 2em; }
    p { margin: 0.8em 0; }
    ul, ol { margin: 0.6em 0 0.6em 2em; }
    li { margin: 0.4em 0; }
    a { color: #007bff; text-decoration: none; }
    a:hover { text-decoration: underline; }
    strong { font-weight: 700; }
    em { font-style: italic; }
    blockquote { color: #666; border-left: 4px solid #eee; margin: 0.8em 0; padding-left: 1em; font-style: italic;}
    .video-wrapper { margin-bottom: 2em; }
    .video-wrapper p { margin-bottom: 0.3em; }
    br { margin-bottom: 8px; }
  </style>
</head>
<body>`, htmlEscape(title))

	fmt.Fprintf(f, `<h1>%s</h1>`, htmlEscape(title))

	if desc != "" {
		fmt.Fprintf(f, `<div class="content">%s</div>`, desc)
	} else {
		fmt.Fprintln(f, `<p><i>Aucun contenu Tiptap</i></p>`)
	}

	if len(videos) > 0 {
		fmt.Fprintln(f, `<h2>Vidéos (offline)</h2>`)
		for _, v := range videos {
			base := filepath.Base(v.Filename)
			fmt.Fprintf(f, `
<div class="video-wrapper">
  <p><b>%s</b> (<i>%s</i>)</p>
  <video controls style="width:100%%; max-width:600px;">
    <source src="%s" type="video/mp4">
    Votre navigateur ne supporte pas la vidéo HTML5.
  </video>
</div>`, htmlEscape(base), htmlEscape(v.URL), htmlEscape(base))
		}
	} else {
		fmt.Fprintln(f, `<p><i>Aucune vidéo dans ce module</i></p>`)
	}

	fmt.Fprintln(f, `</body></html>`)
	return nil
}

// -----------------------------------------------------------------------------
// downloadVideo => yt-dlp
// -----------------------------------------------------------------------------
func downloadVideo(url string, outDir string, idx int) (string, error) {
	final := filepath.Join(outDir, fmt.Sprintf("video-%02d.mp4", idx))
	if fileExistsAndNonZero(final) {
		fmt.Printf("      skipping existing file %s\n", filepath.Base(final))
		return final, nil
	}

	name := fmt.Sprintf("video-%02d.%%(ext)s", idx)
	outputTemplate := filepath.Join(outDir, name)

	cmd := exec.Command("yt-dlp", "-o", outputTemplate, url)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return final, nil
}

// -----------------------------------------------------------------------------
// buildHTMLIndex => index global
// -----------------------------------------------------------------------------
func buildHTMLIndex(all []CourseData, outDir string) {
	fp := filepath.Join(outDir, "index.html")
	f, err := os.Create(fp)
	if err != nil {
		log.Printf("Cannot create index.html: %v\n", err)
		return
	}
	defer f.Close()

	fmt.Fprintln(f, `<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><title>Skool Export Offline</title></head><body>`)
	fmt.Fprintln(f, `<h1>Skool Export Offline</h1>`)
	for _, c := range all {
		cDir := clean(c.Title)
		fmt.Fprintf(f, `<h2>%s</h2><ul>`, htmlEscape(c.Title))
		for _, m := range c.Modules {
			mDir := clean(m.Title)
			link := filepath.Join(cDir, mDir, "module.html")
			fmt.Fprintf(f, `<li><a href="%s">%s</a></li>`, link, htmlEscape(m.Title))
		}
		fmt.Fprintln(f, "</ul>")
	}
	fmt.Fprintln(f, "</body></html>")
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func clean(input string) string {
	s := strings.TrimSpace(input)
	s = removeAccents(s)
	bad := []string{"/", "\\", ":", "?", "*", "\"", "<", ">", "|", "(", ")", "’", "'", "“", "”", "‘", "«", "»", "…", "!", "#", "&", "=", "+"}
	for _, c := range bad {
		s = strings.ReplaceAll(s, c, "-")
	}
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func removeAccents(input string) string {
	var sb strings.Builder
	for _, r := range input {
		if r > 127 {
			switch r {
			case 'é', 'è', 'ê', 'ë':
				r = 'e'
			case 'à', 'â':
				r = 'a'
			case 'ô':
				r = 'o'
			case 'ù', 'û':
				r = 'u'
			case 'î', 'ï':
				r = 'i'
			default:
				r = ' '
			}
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

func uniqueStrings(arr []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range arr {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func fileExistsAndNonZero(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Size() > 0
}
