package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/simplereach/timeutils"
	"github.com/zazab/hierr"
)

func handleTriggers(
	zabbix *Zabbix,
	config *Config,
	args map[string]interface{},
) error {
	var (
		acknowledge    = args["--acknowledge"].(bool)
		words, pattern = parseSearchQuery(args["<pattern>"].([]string))
		confirmation   = !args["--noconfirm"].(bool)

		table = tabwriter.NewWriter(os.Stdout, 1, 4, 2, ' ', 0)
	)

	if len(words) > 0 {
		return fmt.Errorf(
			"unexpected command line token '%s', "+
				"use '/%s' for searching triggers",
			words[0], words[0],
		)
	}

	params, err := parseParams(args)
	if err != nil {
		return err
	}

	triggers, err := zabbix.GetTriggers(params)
	if err != nil {
		return hierr.Errorf(
			err,
			"can't obtain zabbix triggers",
		)
	}

	debugln("* showing triggers table")
	if pattern != "" {
		debugf("** searching %s", pattern)
	}

	identifiers := []string{}
	for _, trigger := range triggers {
		if pattern != "" && !matchPattern(pattern, trigger.String()) {
			continue
		}

		fmt.Fprintf(
			table,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			trigger.LastEvent.ID, trigger.DateTime(),
			trigger.Severity(),
			trigger.StatusProblem(),
			trigger.StatusAcknowledge(),
			trigger.Hostname,
			trigger.Description,
		)

		identifiers = append(identifiers, trigger.LastEvent.ID)
	}

	err = table.Flush()
	if err != nil {
		return err
	}

	if !acknowledge || len(identifiers) == 0 {
		return nil
	}

	if confirmation {
		if !confirmAcknowledge() {
			return nil
		}
	}

	err = zabbix.Acknowledge(identifiers)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, ":: Acknowledged")

	return nil
}

func parseParams(args map[string]interface{}) (Params, error) {
	var (
		severity    = args["--severity"].(int)
		onlyNotAck  = args["--only-nack"].(bool)
		maintenance = args["--maintenance"].(bool)
		problem     = args["--problem"].(bool)
		recent      = args["--recent"].(bool)
		since, _    = args["--since"].(string)
		until, _    = args["--until"].(string)
		sort        = strings.Split(args["--sort"].(string), ",")
		order       = args["--order"].(string)
		limit       = args["--limit"].(string)
	)

	params := Params{
		"sortfield":    sort,
		"sortorder":    order,
		"min_severity": severity,
		"limit":        limit,
	}

	if onlyNotAck {
		params["withLastEventUnacknowledged"] = "1"
	}

	if maintenance {
		params["maintenance"] = "1"
	}

	if recent {
		params["only_true"] = "1"
	}

	if problem {
		params["filter"] = Params{"value": "1"}
	}

	var err error
	if until != "" {
		params["lastChangeTill"], err = parseDateTime(until)
	} else if since != "" {
		params["lastChangeSince"], err = parseDateTime(since)
	}

	return params, err
}

func parseDateTime(value string) (int64, error) {
	date, err := timeutils.ParseDateString(value)
	if err != nil {
		return 0, hierr.Errorf(err, "can't parse datetime '%s'", value)
	}

	return date.Unix(), nil
}

func confirmAcknowledge() bool {
	var value string
	fmt.Fprintf(os.Stderr, "\n:: Proceed with acknowledge? [Y/n]: ")
	fmt.Scanln(&value)
	return value == "" || value == "Y" || value == "y"
}