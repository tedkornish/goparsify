// +build debug

package goparsify

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tedkornish/goparsify/debug"
)

var log io.Writer = nil
var parsers []*debugParser
var pendingOpenLog = ""
var activeParsers []*debugParser
var longestLocation = 0

type debugParser struct {
	Match      string
	Var        string
	Location   string
	Next       Parser
	Cumulative time.Duration
	Self       time.Duration
	SelfStart  time.Time
	Calls      int
	Errors     int
}

func (dp *debugParser) Name() string {
	if len(activeParsers) > 1 && activeParsers[len(activeParsers)-2].Var == dp.Var {
		return dp.Match
	}
	return dp.Var
}

func (dp *debugParser) logf(ps *State, result *Result, format string, args ...interface{}) string {
	buf := &bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("%"+strconv.Itoa(longestLocation)+"s | ", dp.Location))
	buf.WriteString(fmt.Sprintf("%-15s", ps.Preview(15)))
	buf.WriteString(" | ")
	buf.WriteString(strings.Repeat("  ", len(activeParsers)-1))
	buf.WriteString(fmt.Sprintf(format, args...))
	if ps.Errored() {
		buf.WriteString(fmt.Sprintf(" did not find %s", ps.Error.expected))
	} else if result != nil {
		resultStr := strconv.Quote(result.String())
		if len(resultStr) > 20 {
			resultStr = resultStr[0:20]
		}
		buf.WriteString(fmt.Sprintf(" found %s", resultStr))
	}
	buf.WriteRune('\n')
	return buf.String()
}

func (dp *debugParser) logStart(ps *State) {
	if log != nil {
		if pendingOpenLog != "" {
			fmt.Fprint(log, pendingOpenLog)
			pendingOpenLog = ""
		}
		pendingOpenLog = dp.logf(ps, nil, dp.Name()+" {")
	}
}

func (dp *debugParser) logEnd(ps *State, result *Result) {
	if log != nil {
		if pendingOpenLog != "" {
			fmt.Fprintf(log, dp.logf(ps, result, dp.Name()))
			pendingOpenLog = ""
		} else {
			fmt.Fprintf(log, dp.logf(ps, result, "}"))
		}
	}
}

func (dp *debugParser) Parse(ps *State, node *Result) {
	activeParsers = append(activeParsers, dp)
	start := time.Now()
	dp.SelfStart = start

	dp.logStart(ps)
	dp.Next(ps, node)
	dp.logEnd(ps, node)

	dp.Cumulative += time.Since(start)
	dp.Self += time.Since(dp.SelfStart)
	dp.Calls++
	if ps.Errored() {
		dp.Errors++
	}

	activeParsers = activeParsers[0 : len(activeParsers)-1]
}

// NewParser should be called around the creation of every Parser.
// It does nothing normally and should incur no runtime overhead, but when building with -tags debug
// it will instrument every parser to collect valuable timing and debug information.
func NewParser(name string, p Parser) Parser {
	description, location := debug.GetDefinition()

	dp := &debugParser{
		Match:    name,
		Var:      description,
		Location: location,
	}

	dp.Next = func(ps *State, ret *Result) {
		dp.Self += time.Since(dp.SelfStart)

		p(ps, ret)

		dp.SelfStart = time.Now()
	}

	if len(dp.Location) > longestLocation {
		longestLocation = len(dp.Location)
	}

	parsers = append(parsers, dp)
	return dp.Parse
}

// EnableLogging will write logs to the given writer as the next parse happens
func EnableLogging(w io.Writer) {
	log = w
}

// DisableLogging will stop writing logs
func DisableLogging() {
	log = nil
}

// DumpDebugStats will print out the curring timings for each parser if built with -tags debug
func DumpDebugStats() {
	sort.Slice(parsers, func(i, j int) bool {
		return parsers[i].Cumulative >= parsers[j].Cumulative
	})

	fmt.Println()
	fmt.Println("|             var name |              matches |      total time |       self time |      calls |     errors | location  ")
	fmt.Println("| -------------------- | -------------------- | --------------- | --------------- | ---------- | ---------- | ----------")
	for _, parser := range parsers {
		fmt.Printf("| %20s | %20s | %15s | %15s | %10d | %10d | %s\n", parser.Var, parser.Match, parser.Cumulative.String(), parser.Self.String(), parser.Calls, parser.Errors, parser.Location)
	}
}
