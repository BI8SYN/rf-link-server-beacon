package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/BI8SYN/rf-link-server-beacon/internal/app"
	"github.com/BI8SYN/rf-link-server-beacon/internal/config"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func Run(args []string, out, errOut io.Writer, version string) int {
	return RunWithInput(args, os.Stdin, out, errOut, version)
}
func RunWithInput(args []string, input io.Reader, out, errOut io.Writer, version string) int {
	if len(args) == 0 {
		fmt.Fprintln(out, "Usage: rf-link-server-beacon <init|version|status|enroll|wait|doctor|sas-command|sync-identity|once|run|serve> [--config FILE] [--send] [--sas-output FILE]")
		return 0
	}
	cmd := args[0]
	if cmd == "init" {
		return initialize(args[1:], input, out, errOut)
	}

	if cmd == "version" && len(args) == 1 {
		fmt.Fprintln(out, "rf-link-server-beacon", version)
		return 0
	}
	if cmd == "status" && len(args) == 1 {
		fmt.Fprintln(out, `{"phase":"draft-client","broadcasting":false,"enrollment_implemented":true,"production_ready":false,"server_api":"awaiting-integration"}`)
		return 0
	}
	switch cmd {
	case "status", "enroll", "wait", "doctor", "sas-command", "sync-identity", "once", "run", "serve":
	default:
		fmt.Fprintln(errOut, "unknown command")
		return 2
	}
	flags := flag.NewFlagSet(cmd, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	path := flags.String("config", "", "configuration file")
	send := flags.Bool("send", false, "explicitly publish to configured APRS-IS")
	sas := flags.String("sas-output", "", "new absolute SAS configuration file")
	if flags.Parse(args[1:]) != nil || flags.NArg() != 0 {
		fmt.Fprintln(errOut, "invalid arguments; use --config FILE, --send, or --sas-output FILE for the corresponding operation")
		return 2
	}
	if *path == "" {
		fmt.Fprintln(errOut, "--config FILE is required")
		return 2
	}
	if (*send && cmd != "run" && cmd != "serve" && cmd != "once") || (*sas != "" && cmd != "sync-identity") {
		fmt.Fprintln(errOut, "option is not supported for this command")
		return 2
	}
	if (cmd == "run" || cmd == "serve") && !*send {
		fmt.Fprintln(errOut, "run requires --send; use once without --send to verify without APRS transmission")
		return 2
	}
	if cmd == "sync-identity" && *sas == "" {
		fmt.Fprintln(errOut, "sync-identity requires --sas-output with a new absolute path")
		return 2
	}
	cfg, e := config.Load(*path)
	if e != nil {
		fmt.Fprintln(errOut, e)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if cmd == "sas-command" {
		text, err := app.SASCommands(ctx, cfg)
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		if _, err = fmt.Fprint(out, text); err != nil {
			return 1
		}
		return 0
	}
	service, e := app.NewService(cfg)
	if e != nil {
		fmt.Fprintln(errOut, e)
		return 1
	}
	defer service.Close()
	enc := json.NewEncoder(out)
	var value any
	switch cmd {
	case "enroll":
		value, e = service.Enroll(ctx)
	case "status":
		value, e = service.Status(ctx)
	case "wait":
		info, err := service.AwaitIdentity(ctx, func(state string) { enc.Encode(map[string]any{"state": state, "broadcasting": false}) })
		e = err
		if e == nil {
			value = map[string]any{"ready": true, "credential_obtained": true, "certificateFingerprint": info.Fingerprint, "broadcasting": false}
		}
	case "doctor":
		info, err := service.Check(ctx)
		e = err
		if e == nil {
			value = map[string]any{"verified": true, "certificateFingerprint": info.Fingerprint, "uid": info.UID, "station": cfg.Station, "signingEnabled": info.SigningEnabled}
		}
	case "sync-identity":
		e = service.ExportSAS(ctx, *sas)
		value = map[string]any{"generated": e == nil, "applied": false, "note": "Apply this identity to SAS and install the independently trusted root before enabling public MQTT"}
	case "once":
		value, e = service.Cycle(ctx, *send)
	case "run":
		e = service.Run(ctx, func(result app.CycleResult) { enc.Encode(result) })
	case "serve":
		e = service.Serve(ctx,
			func(result app.CycleResult) { enc.Encode(result) },
			func(state string) { enc.Encode(map[string]any{"state": state}) })
	}
	if errors.Is(e, context.Canceled) {
		return 0
	}
	if e != nil {
		fmt.Fprintln(errOut, e)
		return 1
	}
	if value != nil {
		if enc.Encode(value) != nil {
			return 1
		}
	}
	return 0
}
