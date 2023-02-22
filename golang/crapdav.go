package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/emersion/go-ical"
)

func main() {

	icalDir := "/Users/josip/.calendars/josip/2035a302-584c-0187-cef1-db013fc00a83/"
	files, err := ioutil.ReadDir(icalDir)
	if err != nil {
		log.Fatal(err)
	}

	tmpFile, err := os.CreateTemp("", "example")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	for _, f := range files {
		r, err := os.Open(icalDir + f.Name())
		if err != nil {
			log.Fatal(err)
		}

		dec := ical.NewDecoder(r)
		for {
			cal, err := dec.Decode()
			if err == io.EOF {
				break
			} else if err != nil {
				log.Fatal(err)
			}

			for _, child := range cal.Children {
				if child.Name == ical.CompToDo {
					uid := child.Props.Get("uid").Value
					summary := child.Props.Get("summary").Value
					desc, _ := child.Props.Text("description")
					due, err := child.Props.DateTime("due", time.Local)
					dueTime := ""
					if err == nil && !due.IsZero() {
						dueTime = due.String() + "\n"
					}

					if desc != "" {
						desc += "\n"
					}

					fmt.Fprintf(tmpFile, "# %s [%s]\n%s%s\n", summary, uid, dueTime, desc)
				}
			}

			for _, event := range cal.Events() {
				summary, err := event.Props.Text(ical.PropSummary)
				if err != nil {
					log.Fatal(err)
				}
				fmt.Printf("Found event: %v", summary)
			}
		}
		fmt.Println(f.Name())
	}

	if err := tmpFile.Close(); err != nil {
		log.Fatal(err)
	}

	cmd := exec.Command("nvim", tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()

	if err != nil {
		log.Fatal(err)
	}

}
