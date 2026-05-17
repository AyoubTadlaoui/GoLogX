package logx_test

import (
	"bytes"
	"fmt"
	"log/slog"

	"github.com/AyoubTadlaoui/GoLogX/logx"
)

// Pretty handler with deterministic output (NoColor + no timestamp) so the
// Example assertion matches.
func ExampleNew_pretty() {
	var buf bytes.Buffer
	log := logx.New(logx.Options{
		Format:     logx.FormatPretty,
		Output:     &buf,
		NoColor:    true,
		TimeFormat: "", // default; we strip the time below
	})
	log.Info("server up", "port", 8080)

	// Strip the leading "HH:MM:SS.sss " for stable output.
	line := buf.String()
	if i := bytes.IndexByte([]byte(line), ' '); i > 0 {
		line = line[i+1:]
	}
	fmt.Print(line)
	// Output: INF server up port=8080
}

func ExampleNewMultiHandler() {
	var pretty, jsonBuf bytes.Buffer

	multi := logx.NewMultiHandler(
		logx.NewPrettyHandler(&pretty, &logx.PrettyHandlerOptions{NoColor: true, TimeFormat: " "}),
		slog.NewJSONHandler(&jsonBuf, nil),
	)
	log := slog.New(multi)
	log.Info("hit", "path", "/")

	// Both sinks received the same record.
	fmt.Println("pretty has 'hit':", bytes.Contains(pretty.Bytes(), []byte("hit")))
	fmt.Println("json   has 'hit':", bytes.Contains(jsonBuf.Bytes(), []byte(`"msg":"hit"`)))
	// Output:
	// pretty has 'hit': true
	// json   has 'hit': true
}

func ExampleLevelFromString() {
	lvl, _ := logx.LevelFromString("warn")
	fmt.Println(lvl)
	// Output: WARN
}
