package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/google/uuid"
	"github.com/tj/go-naturaldate"
)

func createEvent() *ical.Component {
	uid := uuid.New().String()
	component := ical.NewComponent(ical.CompToDo)
	component.Props.SetText(ical.PropUID, uid)
	component.Props.SetDateTime(ical.PropDateTimeStamp, time.Now().UTC())
	component.Props.SetDateTime(ical.PropCreated, time.Now().UTC())

	return component
}

func updateEvent(event *ical.Component, summary, description, status string, due time.Time) []byte {
	event.Props.SetText(ical.PropStatus, strings.TrimSpace(status))
	event.Props.SetText(ical.PropSummary, strings.TrimSpace(summary))
	event.Props.SetText(ical.PropDescription, strings.TrimSpace(description))
	if !due.IsZero() {
		event.Props.SetDateTime(ical.PropDue, due.UTC())
	}
	event.Props.SetDateTime(ical.PropLastModified, time.Now().UTC())
	return printEvent(event)
}

func printEvent(component *ical.Component) []byte {

	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, "dev.jjanzic.crapdav")
	cal.Children = append(cal.Children, component)

	var buf bytes.Buffer
	encoder := ical.NewEncoder(&buf)
	if err := encoder.Encode(cal); err != nil {
		log.Fatal(err)
	}

	return buf.Bytes()
}

type byEvents []*ical.Component

func (s byEvents) Len() int {
	return len(s)
}
func (s byEvents) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func get(component *ical.Component, name string) string {
	prop := component.Props.Get(name)
	if prop == nil {
		return ""
	}
	return prop.Value
}

func getText(component *ical.Component, name string) string {
	v, err := component.Props.Text(name)
	if err != nil {
		return ""
	}
	return v
}

func (s byEvents) Less(i, j int) bool {
	a := s[i]
	b := s[j]
	aDue := get(a, ical.PropDue)
	bDue := get(b, ical.PropDue)

	aCreated := get(a, ical.PropDateTimeStamp)
	bCreated := get(b, ical.PropDateTimeStamp)

	aStatus := get(a, ical.PropStatus)
	bStatus := get(b, ical.PropStatus)

	if aStatus == bStatus {
		if aDue == bDue {
			return aCreated < bCreated
		}

		if aDue == "" {
			return false
		}
		if bDue == "" {
			return true
		}
		return aDue < bDue
	}

	if aStatus == "COMPLETED" {
		return false
	}
	if bStatus == "COMPLETED" {
		return true
	}
	return aStatus < bStatus
}

func main() {
	icalDir := "/Users/josip/.calendars/josip/2035a302-584c-0187-cef1-db013fc00a83/" // "./test/"
	files, err := ioutil.ReadDir(icalDir)
	if err != nil {
		log.Fatal(err)
	}

	tmpFile, err := os.CreateTemp("", "*.md")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	m := make(map[string]*ical.Component)

	events := make([]*ical.Component, 0)

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
					events = append(events, child)
				}
			}
		}
	}

	sort.Sort(byEvents(events))

	for _, event := range events {
		uid := event.Props.Get(ical.PropUID).Value
		summary := get(event, ical.PropSummary)
		desc := getText(event, ical.PropDescription)
		due, err := event.Props.DateTime("due", time.Local)
		dueTime := ""
		if err == nil && !due.IsZero() {
			dueTime = "@" + due.Local().Format("2006-01-02 15:04") + "\n"
		}

		status := event.Props.Get(ical.PropStatus)

		if status != nil && status.Value == "COMPLETED" {
			summary = "✔️ " + summary
			continue
		}

		if desc != "" {
			desc += "\n"
		}

		m[uid] = event

		fmt.Fprintf(tmpFile, "# %s {#%s}\n%s%s\n", summary, uid, dueTime, desc)
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

	contents, err := os.Open(tmpFile.Name())
	if err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(contents)
	summary := ""
	uid := ""
	description := ""
	status := "NEEDS-ACTION"
	due := time.Time{}

	headersRegex := regexp.MustCompile(`#+ *(✔️)? *(?P<heading>[^{]*) *{?#?(?P<id>[^}]*)`)

	outDir := icalDir // "./test/"
	for scanner.Scan() {
		line := scanner.Text()
		headMatch := headersRegex.FindStringSubmatch(line)
		if len(headMatch) > 0 {
			if summary != "" {
				var component *ical.Component

				if uid == "" {
					component = createEvent()
				} else {
					component = m[uid]
				}

				data := updateEvent(component, summary, description, status, due)

				fileName := outDir + component.Props.Get(ical.PropUID).Value + ".ics"
				err := os.WriteFile(fileName, data, 0644)
				if err != nil {
					fmt.Println("Failed to write")
				}

				summary = ""
				uid = ""
				description = ""
				status = "NEEDS-ACTION"
				due = time.Time{}
			}

			if headMatch[1] == "✔️" {
				status = "COMPLETED"
			}
			summary = headMatch[2]
			uid = headMatch[3]

		} else if len(line) > 0 && line[0] == '@' {
			t, err := time.Parse("2006-01-02 15:04", line[1:])
			if err == nil {
				due = t
				continue
			}
			t, err = naturaldate.Parse(string(line[1:]), time.Now().Local(), naturaldate.WithDirection(naturaldate.Future))
			if err == nil {
				due = t
				continue
			}
			fmt.Println(err)
		} else {
			description += line + "\n"
		}
	}

	// fmt.Printf("summary: %s", summary)

	if summary != "" {
		var component *ical.Component

		if uid == "" {
			component = createEvent()
		} else {
			component = m[uid]
		}

		data := updateEvent(component, summary, description, status, due)
		fileName := outDir + component.Props.Get(ical.PropUID).Value + ".ics"
		err := os.WriteFile(fileName, data, 0644)
		if err != nil {
			fmt.Println("Failed to write")
		}

		summary = ""
		uid = ""
		description = ""
		status = "NEEDS-ACTION"
		due = time.Time{}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}
