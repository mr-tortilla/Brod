//go:build ignore

// Генератор THIRD_PARTY_LICENSES.txt: go run tools/genlicenses/main.go
//
// Собирает лицензии Go-модулей, которые входят в сборку под Linux и Windows,
// и npm-пакетов интерфейса (без dev-зависимостей). Одинаковые тексты лицензий
// печатаются один раз со списком пакетов. Запускать после изменения зависимостей.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const (
	output    = "THIRD_PARTY_LICENSES.txt"
	buildTags = "webkit2_41,with_utls,with_gvisor,badlinkname"
)

var licenseFile = regexp.MustCompile(`(?i)^(licen[cs]e|copying|notice)([.-].*)?$`)

type pkg struct{ name, dir string }

func main() {
	var pkgs []pkg
	seen := map[string]bool{}
	for _, goos := range []string{"linux", "windows"} {
		for _, p := range goModules(goos) {
			if !seen[p.name] {
				seen[p.name] = true
				pkgs = append(pkgs, p)
			}
		}
	}
	pkgs = append(pkgs, npmPackages()...)
	slices.SortFunc(pkgs, func(a, b pkg) int { return strings.Compare(a.name, b.name) })

	// текст лицензии → пакеты с этим текстом
	type group struct {
		names []string
		text  string
	}
	var order []string
	groups := map[string]*group{}
	missing := 0
	for _, p := range pkgs {
		text := licenseText(p.dir)
		if text == "" {
			log.Printf("нет файла лицензии: %s (%s)", p.name, p.dir)
			missing++
			continue
		}
		key := fmt.Sprintf("%x", sha256.Sum256([]byte(text)))
		if groups[key] == nil {
			groups[key] = &group{text: text}
			order = append(order, key)
		}
		groups[key].names = append(groups[key].names, p.name)
	}

	var b strings.Builder
	b.WriteString("Brod использует стороннее программное обеспечение. Ниже — лицензии всех компонентов,\n")
	b.WriteString("которые входят в сборку приложения.\n\n")
	for _, key := range order {
		g := groups[key]
		b.WriteString(strings.Repeat("=", 80) + "\n")
		for _, n := range g.names {
			b.WriteString(n + "\n")
		}
		b.WriteString(strings.Repeat("-", 80) + "\n\n")
		b.WriteString(g.text + "\n\n")
	}
	if err := os.WriteFile(output, []byte(b.String()), 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("%s: %d пакетов, %d уникальных текстов, без лицензии: %d", output, len(pkgs), len(order), missing)
}

// goModules — модули, пакеты которых реально компилируются в бинарник под goos.
func goModules(goos string) []pkg {
	cmd := exec.Command("go", "list", "-deps", "-tags", buildTags,
		"-f", "{{with .Module}}{{if not .Main}}{{.Path}}@{{.Version}}\t{{.Dir}}{{end}}{{end}}", ".")
	cmd.Env = append(os.Environ(), "GOOS="+goos)
	out, err := cmd.Output()
	if err != nil {
		log.Fatalf("go list (%s): %v", goos, err)
	}
	var res []pkg
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		name, dir, ok := strings.Cut(line, "\t")
		if !ok || seen[name] {
			continue
		}
		seen[name] = true
		res = append(res, pkg{name: name, dir: dir})
	}
	return res
}

// npmPackages — production-зависимости фронтенда.
func npmPackages() []pkg {
	cmd := exec.Command("npm", "ls", "--omit=dev", "--all", "--parseable")
	cmd.Dir = "frontend"
	out, err := cmd.Output()
	if err != nil {
		log.Fatalf("npm ls: %v", err)
	}
	root, _ := filepath.Abs("frontend")
	var res []pkg
	for _, dir := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if dir == root {
			continue
		}
		var meta struct{ Name, Version string }
		data, err := os.ReadFile(filepath.Join(dir, "package.json"))
		if err != nil || json.Unmarshal(data, &meta) != nil {
			continue
		}
		res = append(res, pkg{name: "npm:" + meta.Name + "@" + meta.Version, dir: dir})
	}
	return res
}

func licenseText(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var parts []string
	for _, e := range entries {
		if e.IsDir() || !licenseFile.MatchString(e.Name()) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		parts = append(parts, strings.TrimSpace(strings.ReplaceAll(string(data), "\r\n", "\n")))
	}
	return strings.Join(parts, "\n\n")
}
