package main

import (
	"fmt"
	"jjanzic/crapdav"
	"log"
	"os"
	"time"

	"github.com/emersion/go-ical"
	"github.com/urfave/cli"
	// "github.com/dustin/go-humanize"
)


func main() {
	app := cli.NewApp()
	app.Name = "crapdav"
	app.Usage = "ical todo manager"

	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "config", Value: "", Usage: "~/.crapdav.ini"},
	}
	app.Commands = []cli.Command{
		{
			Name:    "edit",
			Aliases: []string{"e"},
			Usage:   "edit the todos in an $EDITOR",
			Action: func(c *cli.Context) error {
				err, config := crapdav.ParseConfig(c.GlobalString("config"))
				if err != nil {
					return err
				}
				return edit(config.Calendar)
			},
		},

		{
			Name:    "print",
			Aliases: []string{"p"},
			Usage:   "print the todos",
			Action: func(c *cli.Context) error {
				err, config := crapdav.ParseConfig(c.GlobalString("config"))
				if err != nil {
					return err
				}
				return print(config.Calendar)
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func print(icalDir string) error {
	if icalDir == "" {
		return fmt.Errorf("no calendar directory setup")
	}
	err, events := crapdav.ParseEvents(icalDir)
	if err != nil {
		return err
	}
	for _, event := range events {
		summary := crapdav.Get(event, ical.PropSummary)
		desc := crapdav.GetText(event, ical.PropDescription)
		due, err := event.Props.DateTime("due", time.Local)
		dueTime := ""
		if err == nil && !due.IsZero() {
			dueTime = "@" + due.Local().Format("Mon 2006-01-02 15:04")
		}

		status := event.Props.Get(ical.PropStatus)

		if status != nil && status.Value == "COMPLETED" {
			summary = "✔️ " + summary
			continue
		}

		if desc != "" {
			desc += "\n"
		}

		fmt.Printf("%s %s\n", summary, dueTime)
	}
	return nil
}

func edit(icalDir string) error {
	if icalDir == "" {
		return fmt.Errorf("no calendar directory setup")
	}
	err, events := crapdav.ParseEvents(icalDir)
	if err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp("", "*.md")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())


	if err := crapdav.IcalToMarkdown(tmpFile, events); err != nil {
		log.Fatal(err)
	}

	if err := tmpFile.Close(); err != nil {
		log.Fatal(err)
	}

	err = crapdav.RunEditor(tmpFile)
	if err != nil {
		return err
	}

	err = crapdav.WriteEvents(icalDir, tmpFile.Name(), events)
	return nil
}

