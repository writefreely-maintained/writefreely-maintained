/*
 * Copyright © 2018-2021 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package postfreely

import (
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/writeas/web-core/l10n"
	"github.com/writeas/web-core/log"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/postfreely/postfreely/config"
)

var (
	templates = map[string]*template.Template{}
	pages     = map[string]*template.Template{}
	userPages = map[string]*template.Template{}
	funcMap   = template.FuncMap{
		"largeNumFmt": largeNumFmt,
		"pluralize":   pluralize,
		"isRTL":       isRTL,
		"isLTR":       isLTR,
		"localstr":    localStr,
		"localhtml":   localHTML,
		"tolower":     strings.ToLower,
		"title":       title,
		"hasPrefix":   strings.HasPrefix,
		"hasSuffix":   strings.HasSuffix,
		"dict":        dict,
	}
)

const (
	templatesDir = "templates"
	pagesDir     = "pages"
)

//go:embed templates
var templatesFS embed.FS

//go:embed pages
var pagesFS embed.FS

//go:embed static
var staticFS embed.FS

func UnpackTemplates() error {
	return errors.Join(
		unpackFS(templatesFS, templatesDir),
		unpackFS(pagesFS, pagesDir),
		unpackFS(staticFS, staticDir),
	)
}

func unpackFS(someFS embed.FS, destPath string) error {
	return fs.WalkDir(someFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return unpackTemplate(someFS, path, d)
	})
}

func unpackTemplate(someFS embed.FS, path string, d fs.DirEntry) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if path == "." {
			return nil
		}
		if d.IsDir() {
			log.Info("creating directory %s", path)
			return os.MkdirAll(path, 0700)
		}
		data, err := someFS.ReadFile(path)
		if err != nil {
			return err
		}
		log.Info("creating file      %s", path)
		return os.WriteFile(path, data, 0600)
	}
	log.Info("leaving existing   %s", path)
	return nil
}

func showUserPage(w http.ResponseWriter, name string, obj interface{}) {
	if obj == nil {
		log.Error("showUserPage: data is nil!")
		return
	}
	if err := userPages[filepath.Join("user", name+".tmpl")].ExecuteTemplate(w, name, obj); err != nil {
		log.Error("Error parsing %s: %v", name, err)
	}
}

func initTemplate(parentDir, name string) {
	if debugging {
		log.Info("  " + filepath.Join(parentDir, templatesDir, name+".tmpl"))
	}

	files := []string{
		filepath.Join(parentDir, templatesDir, name+".tmpl"),
		filepath.Join(parentDir, templatesDir, "software-name.tmpl"),
		filepath.Join(parentDir, templatesDir, "software-url.tmpl"),
		filepath.Join(parentDir, templatesDir, "software-code-url.tmpl"),
		filepath.Join(parentDir, templatesDir, "software-community-url.tmpl"),
		filepath.Join(parentDir, templatesDir, "software-credit.tmpl"),
		filepath.Join(parentDir, templatesDir, "software-versioned.tmpl"),
		filepath.Join(parentDir, templatesDir, "include", "footer.tmpl"),
		filepath.Join(parentDir, templatesDir, "base.tmpl"),
		filepath.Join(parentDir, templatesDir, "user", "include", "silenced.tmpl"),
	}
	if name == "collection" || name == "collection-tags" || name == "chorus-collection" || name == "read" {
		// These pages list out collection posts, so we also parse templatesDir + "include/posts.tmpl"
		files = append(files, filepath.Join(parentDir, templatesDir, "include", "posts.tmpl"))
	}
	if name == "chorus-collection" || name == "chorus-collection-post" {
		files = append(files, filepath.Join(parentDir, templatesDir, "user", "include", "header.tmpl"))
	}
	if name == "collection" || name == "collection-tags" || name == "collection-post" || name == "post" || name == "chorus-collection" || name == "chorus-collection-post" {
		files = append(files, filepath.Join(parentDir, templatesDir, "include", "post-render.tmpl"))
	}
	templates[name] = template.Must(template.New("").Funcs(funcMap).ParseFiles(files...))
}

func initPage(parentDir, path, key string) {
	if debugging {
		log.Info("  [%s] %s", key, path)
	}

	files := []string{
		path,
		filepath.Join(parentDir, templatesDir, "software-name.tmpl"),
		filepath.Join(parentDir, templatesDir, "software-url.tmpl"),
		filepath.Join(parentDir, templatesDir, "software-code-url.tmpl"),
		filepath.Join(parentDir, templatesDir, "software-community-url.tmpl"),
		filepath.Join(parentDir, templatesDir, "software-credit.tmpl"),
		filepath.Join(parentDir, templatesDir, "software-versioned.tmpl"),
		filepath.Join(parentDir, templatesDir, "include", "footer.tmpl"),
		filepath.Join(parentDir, templatesDir, "base.tmpl"),
		filepath.Join(parentDir, templatesDir, "user", "include", "silenced.tmpl"),
	}

	if key == "login.tmpl" || key == "landing.tmpl" || key == "signup.tmpl" {
		files = append(files, filepath.Join(parentDir, templatesDir, "include", "oauth.tmpl"))
	}

	pages[key] = template.Must(template.New("").Funcs(funcMap).ParseFiles(files...))
}

func initUserPage(parentDir, path, key string) {
	if debugging {
		log.Info("  [%s] %s", key, path)
	}

	userPages[key] = template.Must(template.New(key).Funcs(funcMap).ParseFiles(
		path,
		filepath.Join(parentDir, templatesDir, "software-name.tmpl"),
		filepath.Join(parentDir, templatesDir, "software-url.tmpl"),
		filepath.Join(parentDir, templatesDir, "software-code-url.tmpl"),
		filepath.Join(parentDir, templatesDir, "software-community-url.tmpl"),
		filepath.Join(parentDir, templatesDir, "software-credit.tmpl"),
		filepath.Join(parentDir, templatesDir, "software-versioned.tmpl"),
		filepath.Join(parentDir, templatesDir, "user", "include", "header.tmpl"),
		filepath.Join(parentDir, templatesDir, "user", "include", "footer.tmpl"),
		filepath.Join(parentDir, templatesDir, "user", "include", "silenced.tmpl"),
		filepath.Join(parentDir, templatesDir, "user", "include", "nav.tmpl"),
	))
}

// InitTemplates loads all template files from the configured parent dir.
func InitTemplates(cfg *config.Config) error {

	var templatesPath = filepath.Join(cfg.Server.TemplatesParentDir, templatesDir)
	if _, err := os.Stat(templatesPath); os.IsNotExist(err) {
		var path = templatesPath

		s, err := filepath.Abs(templatesPath)
		if nil == err {
			path = s
		}

		return fmt.Errorf("the directory for the 'templates/' templates does not exist — should have been at %q", path)
	}

	log.Info("Loading templates...")
	tmplFiles, err := os.ReadDir(templatesPath)
	if err != nil {
		return err
	}

	for _, f := range tmplFiles {
		if !f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
			parts := strings.Split(f.Name(), ".")
			key := parts[0]
			initTemplate(cfg.Server.TemplatesParentDir, key)
		}
	}

	var pagesPath = filepath.Join(cfg.Server.PagesParentDir, pagesDir)
	if _, err := os.Stat(pagesPath); os.IsNotExist(err) {
		var path = pagesPath

		s, err := filepath.Abs(pagesPath)
		if nil == err {
			path = s
		}

		return fmt.Errorf("the directory for the 'pages/' templates does not exist — should have been at %q", path)
	}

	log.Info("Loading pages...")
	// Initialize all static pages that use the base template
	err = filepath.Walk(pagesPath, func(path string, i os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("problem loading template from %q: %w", path, err)
		}

		if i != nil && !i.IsDir() && !strings.HasPrefix(i.Name(), ".") {
			key := i.Name()
			initPage(cfg.Server.PagesParentDir, path, key)
		}

		return nil
	})
	if err != nil {
		return err
	}

	log.Info("Loading user pages...")
	// Initialize all user pages that use base templates
	err = filepath.Walk(filepath.Join(cfg.Server.TemplatesParentDir, templatesDir, "user"), func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
			corePath := path
			if cfg.Server.TemplatesParentDir != "" {
				corePath = corePath[len(cfg.Server.TemplatesParentDir)+1:]
			}
			parts := strings.Split(corePath, string(filepath.Separator))
			key := f.Name()
			if len(parts) > 2 {
				key = filepath.Join(parts[1], f.Name())
			}
			initUserPage(cfg.Server.TemplatesParentDir, path, key)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// renderPage retrieves the given template and renders it to the given io.Writer.
// If something goes wrong, the error is logged and returned.
func renderPage(w io.Writer, tmpl string, data interface{}) error {
	err := pages[tmpl].ExecuteTemplate(w, "base", data)
	if err != nil {
		log.Error("%v", err)
	}
	return err
}

func largeNumFmt(n int64) string {
	return humanize.Comma(n)
}

func pluralize(singular, plural string, n int64) string {
	if n == 1 {
		return singular
	}
	return plural
}

func isRTL(d string) bool {
	return d == "rtl"
}

func isLTR(d string) bool {
	return d == "ltr" || d == "auto"
}

func localStr(term, lang string) string {
	s := l10n.Strings(lang)[term]
	if s == "" {
		s = l10n.Strings("")[term]
	}
	return s
}

func localHTML(term, lang string) template.HTML {
	s := l10n.Strings(lang)[term]
	if s == "" {
		s = l10n.Strings("")[term]
	}
	s = strings.Replace(s, "write.as", "<a href=\"https://writefreely.org\">writefreely</a>", 1)
	return template.HTML(s)
}

// title is a drop-in replacement for the deprecated strings.Title.
// It uses the root language value, which should provide the same capitalization
// rules as strings.Title did, but with awareness of Unicode punctuation and emoji.
func title(d string) string {
	return cases.Title(language.Und, cases.NoLower).String(d)
}

// from: https://stackoverflow.com/a/18276968/1549194
func dict(values ...interface{}) (map[string]interface{}, error) {
	if len(values)%2 != 0 {
		return nil, errors.New("dict: invalid number of parameters")
	}
	dict := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, errors.New("dict: keys must be strings")
		}
		dict[key] = values[i+1]
	}
	return dict, nil
}
