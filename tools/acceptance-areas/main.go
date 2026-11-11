// Command acceptance-areas decides which acceptance test jobs a pull request must run.
//
// The table in .github/acceptance-areas.yaml splits the repository into areas. A pull
// request that changes only files of some areas runs only the jobs that run the tests of
// those areas, and of the areas that depend on them. A pull request that changes any other
// file runs every job.
//
//	acceptance-areas lint               check the table against the code and the workflow
//	acceptance-areas detect             read changed files on stdin, print GitHub outputs
//	acceptance-areas report             print the jobs of each area
//	acceptance-areas uses               print the files that other areas use, with the areas
//	acceptance-areas verify-ran FILE... compare ran-tests files of a run with the table
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fail("usage: acceptance-areas lint|detect|report|verify-ran")
	}
	m, err := loadModel()
	if err != nil {
		fail(err.Error())
	}
	switch os.Args[1] {
	case "lint":
		errs := m.lint()
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, "error: "+e)
		}
		if len(errs) > 0 {
			os.Exit(1)
		}
		fmt.Println("The area table is complete and consistent.")
	case "detect":
		runDetect(m)
	case "report":
		runReport(m)
	case "uses":
		runUses(m)
	case "verify-ran":
		runVerifyRan(m, os.Args[2:])
	default:
		fail("unknown command " + os.Args[1])
	}
}

func runDetect(m *model) {
	var changed []string
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			changed = append(changed, line)
		}
	}
	code, all, jobs, why := m.detect(changed)
	if all {
		jobs = []string{"*"}
	}
	encoded, _ := json.Marshal(append([]string{}, jobs...))
	fmt.Printf("code=%t\njobs=%s\n", code, encoded)
	fmt.Fprintf(os.Stderr, "%s\njobs: %s\n", why, strings.Join(jobs, ", "))
}

func runReport(m *model) {
	byArea := m.testJobs()
	names := make([]string, 0, len(byArea))
	for a := range byArea {
		names = append(names, a)
	}
	sort.Strings(names)
	for _, a := range names {
		label := a
		if a == "" {
			label = "(shared files: run on every change)"
		}
		fmt.Printf("%s\n  %s\n", label, strings.Join(byArea[a], "\n  "))
	}
}

// runVerifyRan compares a real run with the emulation. A job that ran a test that the
// emulation does not send to it means the table would skip that job when it should not.
func runVerifyRan(m *model, files []string) {
	known := map[string]bool{}
	for _, f := range m.gofiles {
		for _, t := range f.tests {
			known[t] = true
		}
	}
	var errs []string
	for _, file := range files {
		id, ok := jobOfRanFile(filepath.Base(file))
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: no job is known for this file name", file))
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			fail(err.Error())
		}
		for _, line := range strings.Split(string(data), "\n") {
			cols := strings.Split(line, "\t")
			if len(cols) < 2 {
				continue
			}
			top, _, _ := strings.Cut(cols[1], "/")
			if !known[top] {
				continue
			}
			if !contains(jobsFor(m.jobs, top), id) {
				errs = append(errs, fmt.Sprintf("job %s ran %s, but the patterns of the workflow say it should not", id, top))
			}
		}
	}
	errs = unique(errs)
	for _, e := range errs {
		fmt.Fprintln(os.Stderr, "error: "+e)
	}
	if len(errs) > 0 {
		os.Exit(1)
	}
	fmt.Println("Every job ran only tests that the area table sends to it.")
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "error: "+msg)
	os.Exit(1)
}

// runUses lists every file whose code another area uses. Such a file is a candidate for the
// `shared` list, or for a move into the area that owns most of its users.
func runUses(m *model) {
	users := map[string]map[string]bool{}
	for _, e := range refEdges(m.gofiles) {
		from, to := m.ownerOf(e.from), m.ownerOf(e.to)
		if from == "" || to == "" || from == to {
			continue
		}
		if users[e.to] == nil {
			users[e.to] = map[string]bool{}
		}
		users[e.to][from] = true
	}
	files := make([]string, 0, len(users))
	for f := range users {
		files = append(files, f)
	}
	sort.Slice(files, func(i, j int) bool { return len(users[files[i]]) > len(users[files[j]]) })
	for _, f := range files {
		areas := make([]string, 0, len(users[f]))
		for a := range users[f] {
			areas = append(areas, a)
		}
		sort.Strings(areas)
		fmt.Printf("%2d areas  %s (%s)  <- %s\n", len(areas), f, m.ownerOf(f), strings.Join(areas, " "))
	}
}
