package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/shlex"
	"golang.org/x/tools/imports"
	"lesiw.io/defers"
)

var builderr = regexp.MustCompile(`^(\./[^\s:]+):(\d+):(\d+):\s*(.+)$`)
var errEOF = errors.New("bad EOF")

const unused = "declared and not used: "
const foundEOF = "found 'EOF'"

type session struct {
	dir string
	pth string
	src []byte
	off int
	usr strings.Builder
}

func main() {
	defer defers.Run()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		defers.Exit(1)
	}
}

func run() error {
	var s session
	var err error
	if len(os.Args) < 2 {
		dir, err := os.MkdirTemp("", "gofi")
		if err != nil {
			return fmt.Errorf("failed to create temporary directory: %w", err)
		}
		defers.Add(func() { _ = os.RemoveAll(dir) })
		cmd := exec.Command("go", "mod", "init", "gofi.localhost")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf(`failed to run "go mod init": %s`,
				bytes.TrimSpace(out))
		}
		s.pth = filepath.Join(dir, "main.go")
		s.src = []byte("package main")
		s.dir = dir
	} else {
		s.pth = os.Args[1]
		s.src, err = os.ReadFile(s.pth)
		if err != nil {
			return fmt.Errorf("bad file %q: %w", s.pth, err)
		}
		defers.Add(func() { _ = os.WriteFile(s.pth, s.src, 0644) })
	}
	return s.run()
}

func (s *session) run() error {
	r := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		var line string
	read:
		input, err := r.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		input = strings.TrimSpace(input)
		if input == ".quit" || input == ".exit" {
			break
		}
		if strings.HasPrefix(input, ":") {
			argv, err := shlex.Split(input[1:])
			if err != nil || len(argv) == 0 {
				fmt.Fprintf(os.Stderr, "bad command: %s", err)
				continue
			}
			cmd := exec.Command(argv[0], argv[1:]...)
			cmd.Dir = s.dir
			if out, err := cmd.CombinedOutput(); err != nil {
				fmt.Fprintf(os.Stderr, "command failed: ")
				if ee := new(exec.ExitError); errors.As(err, &ee) {
					fmt.Fprintf(os.Stderr, "%s\n",
						bytes.TrimSuffix(out, []byte("\n")))
				} else {
					fmt.Fprintf(os.Stderr, "%s\n", err)
				}
			}
		} else {
			line += input
			if err := s.exec(line); errors.Is(err, errEOF) {
				goto read
			} else if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}
	return nil
}

func (s *session) exec(input string) error {
	var fixes strings.Builder
	input = input + "\n"
rerun:
	if err := s.write(input + fixes.String()); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	buf, err := imports.Process(s.pth, nil, nil)
	if err != nil && strings.Contains(err.Error(), foundEOF) {
		return errEOF
	} else if err != nil {
		return fmt.Errorf("failed to process imports: %w", err)
	}
	if err := os.WriteFile(s.pth, buf, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	cmd := exec.Command("go", "run", s.pth)
	cmd.Dir = s.dir
	buf, err = cmd.CombinedOutput()
	output := string(buf)
	if err != nil {
		lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
		if strings.HasPrefix(lines[len(lines)-1], "exit status ") {
			// The program errored, so return its error.
			return errors.New(strings.TrimSuffix(s.newLines(output), "\n"))
		}
		// This is a compile error, so try to fix it.
		var fixed bool
		for _, line := range strings.Split(output, "\n") {
			if m := builderr.FindStringSubmatch(line); m != nil {
				if strings.HasPrefix(m[4], unused) {
					fixed = true
					fixes.WriteString("_ = " + m[4][len(unused):] + "\n")
				}
			}
		}
		if fixed {
			goto rerun
		}
		return errors.New(strings.TrimSuffix(output, "\n"))
	}
	s.usr.WriteString(input)
	out := strings.TrimSuffix(s.newLines(output), "\n")
	if out != "" {
		fmt.Println(out)
	}
	s.off = strings.Count(output, "\n")
	return nil
}

func (s *session) write(input string) (err error) {
	f, err := os.Create(s.pth)
	if err != nil {
		return err
	}
	defer f.Close()
	w := func(str string) {
		if err != nil {
			return
		}
		_, err = f.WriteString(str)
	}
	w(string(s.src))
	w("\nfunc main() {\n")
	w(s.usr.String())
	w(input)
	w("}")
	return
}

func (s *session) newLines(output string) string {
	target := len(output)
	var count int
	for i, r := range output {
		if count >= s.off {
			target = i
			break
		}
		if r == '\n' {
			count++
		}
	}
	return stringSlice(output, target, len(output))
}

func stringSlice(s string, start, end int) string {
	runes := []rune(s)
	if start < 0 || start > len(runes) {
		return ""
	} else if end < 0 || end > len(runes) {
		return ""
	} else if start > end {
		return ""
	}
	return string(runes[start:end])
}
